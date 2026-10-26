package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresVaultPublications grava a publicação dos cofres na transação do ator, sob
// RLS: só owner e admin membros do cofre escrevem.
type PostgresVaultPublications struct {
	pool *pgxpool.Pool
}

// NewPostgresVaultPublications monta o repositório sobre um pool já aberto.
func NewPostgresVaultPublications(pool *pgxpool.Pool) *PostgresVaultPublications {
	return &PostgresVaultPublications{pool: pool}
}

// PublishTx grava a publicação e a auditoria juntas. Devolve publication.ErrSlugTaken
// quando outro cofre tem o slug e notes.ErrVaultForbidden quando o ator não enxerga o
// cofre ou não pode publicá-lo.
func (r *PostgresVaultPublications) PublishTx(ctx context.Context, p publication.Publication, event auth.AuditEventRecord) (publication.Publication, error) {
	var out publication.Publication
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		row, err := q.UpsertVaultPublication(ctx, gen.UpsertVaultPublicationParams{
			VaultID: p.VaultID, Slug: p.Slug, Title: p.Title, PublishedBy: p.PublishedBy, PublishedAt: p.PublishedAt,
		})
		if err != nil {
			return publishErr(err, p)
		}
		out = publication.Publication(row)
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
	return out, err
}

// publishErr traduz a gravação recusada: cofre fora do alcance não insere linha, e o
// RLS barra quem não é owner nem admin (42501).
func publishErr(err error, p publication.Publication) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows), pgErrorCode(err) == insufficientPrivilegeCode:
		return fmt.Errorf("%w: publicar o cofre %q", notes.ErrVaultForbidden, p.VaultID)
	case isUniqueConstraintViolation(err):
		return fmt.Errorf("%w: %q", publication.ErrSlugTaken, p.Slug)
	}
	return fmt.Errorf("publicar o cofre %q com o slug %q: %w", p.VaultID, p.Slug, err)
}

// UnpublishTx apaga a publicação do cofre e audita junto, ou devolve
// publication.ErrNotPublished quando o ator não enxerga publicação nenhuma nele.
func (r *PostgresVaultPublications) UnpublishTx(ctx context.Context, vaultID string, event auth.AuditEventRecord) error {
	return inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		rows, err := q.DeleteVaultPublication(ctx, vaultID)
		if err != nil {
			return fmt.Errorf("despublicar o cofre %q: %w", vaultID, err)
		}
		if rows == 0 {
			return fmt.Errorf("%w: cofre %q", publication.ErrNotPublished, vaultID)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
}

// Publication lê a publicação do cofre vaultID, ou devolve publication.ErrNotPublished.
func (r *PostgresVaultPublications) Publication(ctx context.Context, vaultID string) (publication.Publication, error) {
	var out publication.Publication
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		row, err := q.GetVaultPublication(ctx, vaultID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: cofre %q", publication.ErrNotPublished, vaultID)
		}
		out = publication.Publication(row)
		return err
	})
	return out, err
}
