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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/drain"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
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

// runEmbedder roda o indexador de embedding em laço (ADR-0053). Sem provedor, desliga
// o modelo anterior, se havia, e fica ocioso até ser encerrado.
func runEmbedder(ctx context.Context, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, emb, err := loadEmbedderConfig()
	if err != nil {
		return err
	}
	provider, closeProvider, err := newEmbeddingProvider(ctx, emb)
	if err != nil {
		return err
	}
	defer func() { _ = closeProvider() }()
	pool, err := storage.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer pool.Close()
	if provider == nil {
		return idleWithoutProvider(ctx, logger, pool)
	}
	return embedNotes(ctx, logger, pool, provider, emb.ChunkTokens, cfg.IndexInterval)
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

// idleWithoutProvider registra a instalação sem modelo, o que apaga os vetores e as
// ligações por sentido de um modelo anterior, e espera o encerramento.
func idleWithoutProvider(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool) error {
	if _, err := storage.UseEmbeddingModel(ctx, pool, config.EmbeddingNone, 0); err != nil {
		return err
	}
	logger.Warn("indexador de embedding ocioso", "motivo", "MARKUPP_EMBEDDING_PROVIDER vazia, sem ligacao por sentido")
	<-ctx.Done()
	return nil
}

// embedNotes registra o modelo e refaz os vetores pendentes em laço. O erro de um
// lote vai para o log e espera o intervalo, como no indexador de texto.
func embedNotes(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool, provider embedding.Provider, chunkTokens int, interval time.Duration) error {
	use := func(ctx context.Context, name string, dimension int) (bool, error) {
		return storage.UseEmbeddingModel(ctx, pool, name, dimension)
	}
	changed, err := embedding.Activate(ctx, provider, use)
	if err != nil {
		return err
	}
	logger.Info("indexador de embedding em laco", "modelo", provider.Name(), "modelo_trocado", changed, "intervalo", interval.String())
	indexer := embedding.NewIndexer(storage.NewPostgresEmbeddingQueue(pool), provider, chunkTokens, embedBatchSize)
	next := func(ctx context.Context) (bool, error) {
		found, err := indexer.IndexNext(ctx)
		if err != nil {
			logger.Error("refazer vetores do proximo lote", "err", err)
			return false, nil
		}
		return found, nil
	}
	return drain.Run(ctx, next, drain.WaitInterval(interval))
}
