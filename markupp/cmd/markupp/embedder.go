package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

const (
	// embedBatchSize limita as notas de um lote do indexador de embedding: o modelo
	// roda fora da transação, e o lote menor perde menos trabalho numa nota editada.
	embedBatchSize = 16
	// remoteEmbedTimeout limita um pedido ao endpoint remoto, e modelDownloadTimeout o
	// download de um arquivo do modelo local, que tem centenas de MB.
	remoteEmbedTimeout   = 2 * time.Minute
	modelDownloadTimeout = 30 * time.Minute
)

// runEmbedder roda o indexador de embedding em laço (ADR-0053). O provedor é o de
// MARKUPP_EMBEDDING_PROVIDER quando quem hospeda o define, e senão o escolhido no banco,
// relido a cada intervalo (ADR-0057).
func runEmbedder(ctx context.Context, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, emb, err := loadEmbedderConfig()
	if err != nil {
		return err
	}
	pool, err := storage.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer pool.Close()
	report := storage.NewIndexerInstanceEmbedding(pool)
	source, err := newSettingSource(ctx, cfg, emb, pool, report)
	if err != nil {
		return err
	}
	return newEmbeddingSupervisor(pool, emb, source, report, cfg.IndexInterval, logger).Run(ctx, cfg.IndexInterval)
}

func loadEmbedderConfig() (config.Config, config.Embedding, error) {
	cfg, err := config.LoadIndexer(os.LookupEnv)
	if err != nil {
		return cfg, config.Embedding{}, fmt.Errorf("load indexer config: %w", err)
	}
	emb, err := config.LoadEmbedding(os.LookupEnv)
	if err != nil {
		return cfg, emb, fmt.Errorf("load embedding config: %w", err)
	}
	return cfg, emb, nil
}

func newEmbeddingSupervisor(pool *pgxpool.Pool, emb config.Embedding, source embedding.SettingSource, report embedding.StatusReporter, retryBase time.Duration, logger *slog.Logger) *embedding.Supervisor {
	return embedding.NewSupervisor(embedding.SupervisorDeps{
		Source: source, Status: report, Queue: storage.NewPostgresEmbeddingQueue(pool),
		Open: func(ctx context.Context, s embedding.Setting) (embedding.Provider, func() error, error) {
			return newEmbeddingProvider(ctx, config.Embedding{Provider: s.Provider, URL: s.URL, Model: s.Model, APIKey: s.APIKey, ModelDir: emb.ModelDir})
		},
		Use: func(ctx context.Context, name string, dimension int) (bool, error) {
			return storage.UseEmbeddingModel(ctx, pool, name, dimension)
		},
		HostLocked: emb.HostLocked, HostProvider: emb.Provider, ChunkTokens: emb.ChunkTokens, BatchSize: embedBatchSize, Logger: logger,
		RetryBase: retryBase,
	})
}

// hostSetting é o provedor fixo de quem hospeda, das variáveis MARKUPP_EMBEDDING_*.
type hostSetting struct{ setting embedding.Setting }

func (h hostSetting) Current(context.Context) (embedding.Setting, error) { return h.setting, nil }

// storedSetting é o provedor escolhido no banco, com a chave decifrada.
type storedSetting struct{ svc *instance.EmbeddingService }

func (s storedSetting) Current(ctx context.Context) (embedding.Setting, error) {
	c, err := s.svc.PlainChoice(ctx)
	if err != nil {
		return embedding.Setting{}, err
	}
	return embedding.Setting{Provider: c.Provider, URL: c.URL, Model: c.Model, APIKey: c.APIKey, Revision: c.UpdatedAt}, nil
}

// newSettingSource escolhe de onde vem o provedor. Só a escolha do banco precisa da
// chave de cifra, que pode ser gerada no banco na primeira vez.
func newSettingSource(ctx context.Context, cfg config.Config, emb config.Embedding, pool *pgxpool.Pool, store instance.EmbeddingStore) (embedding.SettingSource, error) {
	if emb.HostLocked {
		return hostSetting{embedding.Setting{Provider: emb.Provider, URL: emb.URL, Model: emb.Model, APIKey: emb.APIKey}}, nil
	}
	box, err := newSecretBox(ctx, cfg.SecretKey, storage.NewPostgresInstanceSecrets(pool))
	if err != nil {
		return nil, fmt.Errorf("prepare the key-encryption secret: %w", err)
	}
	return storedSetting{instance.NewEmbeddingService(store, box)}, nil
}

// newEmbeddingProvider monta o provedor da config, nil sem provedor.
func newEmbeddingProvider(ctx context.Context, emb config.Embedding) (embedding.Provider, func() error, error) {
	switch emb.Provider {
	case config.EmbeddingOpenAICompat:
		client := &http.Client{Timeout: remoteEmbedTimeout}
		return embedding.NewOpenAICompatProvider(client, emb.URL, emb.Model, emb.APIKey), noClose, nil
	case config.EmbeddingLocal:
		client := &http.Client{Timeout: modelDownloadTimeout}
		return embedding.OpenLocalProvider(ctx, embedding.NewHTTPModelFetcher(client), emb.ModelDir)
	}
	return nil, noClose, nil
}

func noClose() error { return nil }
