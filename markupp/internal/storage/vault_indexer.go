package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresVaultIndexer mantém o índice léxico dos cofres, um cofre por chamada.
type PostgresVaultIndexer struct {
	pool         *pgxpool.Pool
	searchConfig string
}

// NewPostgresVaultIndexer monta o indexador com a configuração de texto
// searchConfig, que o cast para regconfig precisa resolver.
func NewPostgresVaultIndexer(ctx context.Context, pool *pgxpool.Pool, searchConfig string) (*PostgresVaultIndexer, error) {
	if _, err := gen.New(pool).ResolveSearchConfig(ctx, searchConfig); err != nil {
		return nil, fmt.Errorf("configuracao de texto %q nao resolvida, esperado nome de pg_ts_config como simple ou pg_catalog.portuguese: %w", searchConfig, err)
	}
	return &PostgresVaultIndexer{pool: pool, searchConfig: searchConfig}, nil
}

// IndexNextVault reserva um cofre com nota pendente, indexa as notas pendentes dele
// com index e grava o resultado na mesma transação. Devolve false quando não há
// cofre pendente livre.
func (x *PostgresVaultIndexer) IndexNextVault(ctx context.Context, index indexing.IndexFunc) (bool, error) {
	found := false
	err := inTx(ctx, x.pool, func(q *gen.Queries) error {
		vault, err := q.ReservePendingVault(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reservar cofre pendente: %w", err)
		}
		found = true
		return x.indexVault(ctx, q, vault, index)
	})
	return found, err
}

func (x *PostgresVaultIndexer) indexVault(ctx context.Context, q *gen.Queries, vault gen.ReservePendingVaultRow, index indexing.IndexFunc) error {
	if err := q.EnterIndexer(ctx, gen.EnterIndexerParams{TenantID: vault.TenantID, VaultID: vault.VaultID}); err != nil {
		return fmt.Errorf("entrar no cofre %q do tenant %q: %w", vault.VaultID, vault.TenantID, err)
	}
	rows, err := q.ListPendingNotes(ctx, vault.VaultID)
	if err != nil {
		return fmt.Errorf("listar notas pendentes do cofre %q: %w", vault.VaultID, err)
	}
	cs := toChangeset(vault, rows)
	if err := x.saveDocuments(ctx, q, cs, index(cs)); err != nil {
		return err
	}
	return dequeue(ctx, q, vault.VaultID, rows)
}

func toChangeset(vault gen.ReservePendingVaultRow, rows []gen.ListPendingNotesRow) indexing.Changeset {
	cs := indexing.Changeset{TenantID: vault.TenantID, VaultID: vault.VaultID, Notes: make([]indexing.NoteChange, 0, len(rows))}
	for _, row := range rows {
		cs.Notes = append(cs.Notes, indexing.NoteChange{NoteID: row.ID, Content: row.Content})
	}
	return cs
}

// saveDocuments grava os documentos num comando só. Exige um documento por nota do
// lote: nota sem documento ficaria pendente para sempre.
func (x *PostgresVaultIndexer) saveDocuments(ctx context.Context, q *gen.Queries, cs indexing.Changeset, docs []indexing.SearchDocument) error {
	if err := matchBatch(cs, docs); err != nil {
		return err
	}
	params := gen.SaveSearchDocumentsParams{SearchConfig: x.searchConfig, NoteIds: make([]string, 0, len(docs)), Bodies: make([]string, 0, len(docs))}
	for _, doc := range docs {
		params.NoteIds = append(params.NoteIds, doc.NoteID)
		params.Bodies = append(params.Bodies, doc.Text)
	}
	if err := q.SaveSearchDocuments(ctx, params); err != nil {
		return fmt.Errorf("gravar indice das %d notas do cofre %q: %w", len(docs), cs.VaultID, err)
	}
	return nil
}

// matchBatch confere que docs tem exatamente um documento por nota de cs.
func matchBatch(cs indexing.Changeset, docs []indexing.SearchDocument) error {
	missing := make(map[string]bool, len(cs.Notes))
	for _, note := range cs.Notes {
		missing[note.NoteID] = true
	}
	for _, doc := range docs {
		if !missing[doc.NoteID] {
			return fmt.Errorf("documento da nota %q fora do lote do cofre %q, esperado um documento por nota pendente", doc.NoteID, cs.VaultID)
		}
		delete(missing, doc.NoteID)
	}
	for id := range missing {
		return fmt.Errorf("nota %q do cofre %q sem documento, esperado um documento por nota pendente", id, cs.VaultID)
	}
	return nil
}

func dequeue(ctx context.Context, q *gen.Queries, vaultID string, rows []gen.ListPendingNotesRow) error {
	params := gen.DequeueNotesParams{NoteIds: make([]string, 0, len(rows)), Revisions: make([]int64, 0, len(rows))}
	for _, row := range rows {
		params.NoteIds = append(params.NoteIds, row.ID)
		params.Revisions = append(params.Revisions, row.Revision)
	}
	if err := q.DequeueNotes(ctx, params); err != nil {
		return fmt.Errorf("tirar da fila as %d notas do cofre %q: %w", len(rows), vaultID, err)
	}
	return nil
}

// RequeueNotes põe de volta na fila do indexador as notas do cofre vaultID, ou de
// todos os cofres da instalação com vaultID vazio, e devolve quantas entraram. Roda
// no papel da conexão, que precisa ser o dono do banco.
func RequeueNotes(ctx context.Context, pool *pgxpool.Pool, vaultID string) (int64, error) {
	n, err := gen.New(pool).RequeueNotes(ctx, pgtype.Text{String: vaultID, Valid: vaultID != ""})
	if err != nil {
		return 0, fmt.Errorf("reenfileirar as notas do cofre %q (vazio para todos), esperado papel dono do banco: %w", vaultID, err)
	}
	return n, nil
}
