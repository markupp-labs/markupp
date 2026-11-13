package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresInstanceEmbedding lê e grava a linha do embedding da instância (ADR-0057),
// sempre num papel sem privilégio: markupp_app na API, que grava a escolha, e
// markupp_indexer no indexador de embedding, que informa a trava e o estado do modelo.
type PostgresInstanceEmbedding struct {
	pool  *pgxpool.Pool
	enter func(ctx context.Context, pool *pgxpool.Pool, fn func(q *gen.Queries) error) error
}

var _ instance.EmbeddingStore = (*PostgresInstanceEmbedding)(nil)

// NewAppInstanceEmbedding monta o repositório da API, que age na transação do ator.
func NewAppInstanceEmbedding(pool *pgxpool.Pool) *PostgresInstanceEmbedding {
	return &PostgresInstanceEmbedding{pool: pool, enter: inActorTx}
}

// NewIndexerInstanceEmbedding monta o repositório do indexador de embedding.
func NewIndexerInstanceEmbedding(pool *pgxpool.Pool) *PostgresInstanceEmbedding {
	return &PostgresInstanceEmbedding{pool: pool, enter: inInstanceIndexerTx}
}

// inInstanceIndexerTx roda fn no papel markupp_indexer, sem tenant nem cofre: a linha
// da instância não tem RLS.
func inInstanceIndexerTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *gen.Queries) error) error {
	return inTx(ctx, pool, func(q *gen.Queries) error {
		if err := q.EnterIndexer(ctx, gen.EnterIndexerParams{}); err != nil {
			return fmt.Errorf("enter the indexer role: %w", err)
		}
		return fn(q)
	})
}

// LoadEmbedding lê a escolha, a trava de quem hospeda e o estado do modelo.
func (r *PostgresInstanceEmbedding) LoadEmbedding(ctx context.Context) (instance.EmbeddingRecord, error) {
	var row gen.GetInstanceEmbeddingRow
	err := r.enter(ctx, r.pool, func(q *gen.Queries) error {
		var qErr error
		row, qErr = q.GetInstanceEmbedding(ctx)
		return qErr
	})
	if err != nil {
		return instance.EmbeddingRecord{}, fmt.Errorf("read instance embedding: %w", err)
	}
	return toEmbeddingRecord(row), nil
}

func toEmbeddingRecord(row gen.GetInstanceEmbeddingRow) instance.EmbeddingRecord {
	record := instance.EmbeddingRecord{
		Choice:     instance.EmbeddingChoice{Provider: row.Provider, URL: row.Url, Model: row.Model, SealedAPIKey: row.ApiKeySealed, UpdatedAt: row.UpdatedAt},
		HostLocked: row.HostLocked, HostProvider: row.HostProvider,
		Model: instance.ModelStatus{State: row.ModelState, Error: row.ModelError},
	}
	if row.ModelStateAt.Valid {
		at := row.ModelStateAt.Time
		record.Model.ChangedAt = &at
	}
	return record
}

// SaveEmbeddingChoice grava a escolha e muda a revisão, que o indexador acompanha.
func (r *PostgresInstanceEmbedding) SaveEmbeddingChoice(ctx context.Context, choice instance.EmbeddingChoice) error {
	err := r.enter(ctx, r.pool, func(q *gen.Queries) error {
		_, qErr := q.SaveInstanceEmbeddingChoice(ctx, gen.SaveInstanceEmbeddingChoiceParams{
			Provider: choice.Provider, Url: choice.URL, Model: choice.Model, ApiKeySealed: choice.SealedAPIKey,
		})
		return qErr
	})
	if err != nil {
		return fmt.Errorf("save instance embedding provider %q: %w", choice.Provider, err)
	}
	return nil
}

// ReportEmbeddingHost informa se quem hospeda travou o provedor, e qual.
func (r *PostgresInstanceEmbedding) ReportEmbeddingHost(ctx context.Context, locked bool, provider string) error {
	err := r.enter(ctx, r.pool, func(q *gen.Queries) error {
		return q.ReportInstanceEmbeddingHost(ctx, gen.ReportInstanceEmbeddingHostParams{HostLocked: locked, HostProvider: provider})
	})
	if err != nil {
		return fmt.Errorf("report embedding host lock %t with provider %q: %w", locked, provider, err)
	}
	return nil
}

// ReportModelStatus informa o estado do modelo, com a mensagem do erro quando há.
func (r *PostgresInstanceEmbedding) ReportModelStatus(ctx context.Context, state, message string) error {
	err := r.enter(ctx, r.pool, func(q *gen.Queries) error {
		return q.ReportInstanceEmbeddingModel(ctx, gen.ReportInstanceEmbeddingModelParams{ModelState: state, ModelError: message})
	})
	if err != nil {
		return fmt.Errorf("report embedding model state %q: %w", state, err)
	}
	return nil
}
