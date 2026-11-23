package storage

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresEmbeddingQueue é a fila do indexador de embedding no banco.
type PostgresEmbeddingQueue struct{ pool *pgxpool.Pool }

var _ embedding.Queue = (*PostgresEmbeddingQueue)(nil)

// NewPostgresEmbeddingQueue monta a fila sobre pool, que conecta como dono do banco.
func NewPostgresEmbeddingQueue(pool *pgxpool.Pool) *PostgresEmbeddingQueue {
	return &PostgresEmbeddingQueue{pool: pool}
}

// UseEmbeddingModel registra o modelo name, de vetores com dimension posições, e
// devolve true quando ele é outro: os vetores e as ligações por sentido do anterior
// saem e todas as notas voltam para a fila. Dimensão zero é a instalação sem provedor.
func UseEmbeddingModel(ctx context.Context, pool *pgxpool.Pool, name string, dimension int) (bool, error) {
	changed, err := gen.New(pool).UseEmbeddingModel(ctx, gen.UseEmbeddingModelParams{Name: name, Dimension: toInt32(dimension)})
	if err != nil {
		return false, fmt.Errorf("use embedding model %q with %d dimensions: expected database owner role and pgvector: %w", name, dimension, err)
	}
	return changed, nil
}

// NextBatch escolhe ao acaso um cofre com nota pendente e lê até limit notas dele.
func (x *PostgresEmbeddingQueue) NextBatch(ctx context.Context, limit int) (embedding.PendingBatch, bool, error) {
	var batch embedding.PendingBatch
	found := false
	err := inTx(ctx, x.pool, func(q *gen.Queries) error {
		vault, err := q.PickEmbeddingVault(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("pick vault with pending embeddings: %w", err)
		}
		found, batch = true, embedding.PendingBatch{TenantID: vault.TenantID, VaultID: vault.VaultID}
		batch.Notes, err = listPendingEmbeddingNotes(ctx, q, batch, limit)
		return err
	})
	return batch, found, err
}

func listPendingEmbeddingNotes(ctx context.Context, q *gen.Queries, batch embedding.PendingBatch, limit int) ([]embedding.PendingNote, error) {
	if err := enterEmbeddingVault(ctx, q, batch); err != nil {
		return nil, err
	}
	rows, err := q.ListPendingEmbeddingNotes(ctx, gen.ListPendingEmbeddingNotesParams{VaultID: batch.VaultID, BatchLimit: toInt32(limit)})
	if err != nil {
		return nil, fmt.Errorf("list pending embeddings of vault %q: %w", batch.VaultID, err)
	}
	return convertRows(rows, func(r gen.ListPendingEmbeddingNotesRow) embedding.PendingNote {
		return embedding.PendingNote{NoteID: r.ID, Content: r.Content, ContentHash: r.ContentSha256, Revision: r.Revision}
	}), nil
}

func enterEmbeddingVault(ctx context.Context, q *gen.Queries, batch embedding.PendingBatch) error {
	return enterVault(ctx, q, gen.ReservePendingVaultRow{VaultID: batch.VaultID, TenantID: batch.TenantID})
}

// SaveVectors troca os vetores das notas do lote, tira da fila as revisões lidas e
// marca o cofre para o segundo estágio refazer as ligações por sentido.
func (x *PostgresEmbeddingQueue) SaveVectors(ctx context.Context, batch embedding.PendingBatch, vectors []embedding.NoteVector) error {
	return inTx(ctx, x.pool, func(q *gen.Queries) error {
		if err := enterEmbeddingVault(ctx, q, batch); err != nil {
			return err
		}
		if err := replaceNoteEmbeddings(ctx, q, batch.VaultID, vectors); err != nil {
			return err
		}
		if err := dequeueEmbeddings(ctx, q, batch.VaultID, vectors); err != nil {
			return err
		}
		if err := q.MarkSecondStage(ctx, batch.VaultID); err != nil {
			return fmt.Errorf("mark vault %q for the second stage: %w", batch.VaultID, err)
		}
		return nil
	})
}

func replaceNoteEmbeddings(ctx context.Context, q *gen.Queries, vaultID string, vectors []embedding.NoteVector) error {
	var p gen.SaveNoteEmbeddingsParams
	ids := make([]string, 0, len(vectors))
	for _, v := range vectors {
		ids = append(ids, v.Note.NoteID)
		if v.Vector != nil {
			p.NoteIds = append(p.NoteIds, v.Note.NoteID)
			p.ContentHashes = append(p.ContentHashes, v.Note.ContentHash)
			p.Embeddings = append(p.Embeddings, vectorText(v.Vector))
		}
	}
	if err := q.DeleteNoteEmbeddings(ctx, ids); err != nil {
		return fmt.Errorf("delete embeddings of %d notes in vault %q: %w", len(ids), vaultID, err)
	}
	if err := q.SaveNoteEmbeddings(ctx, p); err != nil {
		return fmt.Errorf("save %d embeddings in vault %q: %w", len(p.NoteIds), vaultID, err)
	}
	return nil
}

func dequeueEmbeddings(ctx context.Context, q *gen.Queries, vaultID string, vectors []embedding.NoteVector) error {
	var p gen.DequeueEmbeddingNotesParams
	for _, v := range vectors {
		p.NoteIds = append(p.NoteIds, v.Note.NoteID)
		p.Revisions = append(p.Revisions, v.Note.Revision)
	}
	if err := q.DequeueEmbeddingNotes(ctx, p); err != nil {
		return fmt.Errorf("dequeue %d embeddings of vault %q: %w", len(vectors), vaultID, err)
	}
	return nil
}

// vectorText escreve o vetor no formato de texto do pgvector, [x,y,...].
func vectorText(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(x), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}
