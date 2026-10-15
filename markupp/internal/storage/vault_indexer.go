package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
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
// searchConfig, que precisa existir em pg_ts_config.
func NewPostgresVaultIndexer(ctx context.Context, pool *pgxpool.Pool, searchConfig string) (*PostgresVaultIndexer, error) {
	exists, err := gen.New(pool).SearchConfigExists(ctx, searchConfig)
	if err != nil {
		return nil, fmt.Errorf("consultar configuracao de texto %q: %w", searchConfig, err)
	}
	if !exists {
		return nil, fmt.Errorf("configuracao de texto %q inexistente, esperado um cfgname de pg_ts_config como simple ou portuguese", searchConfig)
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
	rows, err := q.ListPendingNotes(ctx)
	if err != nil {
		return fmt.Errorf("listar notas pendentes do cofre %q: %w", vault.VaultID, err)
	}
	cs, revisions := toChangeset(vault, rows)
	return x.saveDocuments(ctx, q, index(cs), revisions)
}

// toChangeset monta o lote do cofre e guarda a revisão lida de cada nota, que é a
// que o índice registra: escrita feita durante a indexação deixa a nota pendente.
func toChangeset(vault gen.ReservePendingVaultRow, rows []gen.ListPendingNotesRow) (indexing.Changeset, map[string]int64) {
	cs := indexing.Changeset{TenantID: vault.TenantID, VaultID: vault.VaultID, Notes: make([]indexing.NoteChange, 0, len(rows))}
	revisions := make(map[string]int64, len(rows))
	for _, row := range rows {
		cs.Notes = append(cs.Notes, indexing.NoteChange{NoteID: row.ID, Content: row.Content})
		revisions[row.ID] = row.Revision
	}
	return cs, revisions
}

func (x *PostgresVaultIndexer) saveDocuments(ctx context.Context, q *gen.Queries, docs []indexing.SearchDocument, revisions map[string]int64) error {
	for _, doc := range docs {
		err := q.UpsertSearchDocument(ctx, gen.UpsertSearchDocumentParams{
			NoteID: doc.NoteID, Revision: revisions[doc.NoteID], SearchConfig: x.searchConfig, Body: doc.Text,
		})
		if err != nil {
			return fmt.Errorf("gravar indice da nota %q: %w", doc.NoteID, err)
		}
	}
	return nil
}
