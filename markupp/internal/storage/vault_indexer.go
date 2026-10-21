package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

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
	logger       *slog.Logger
}

// NewPostgresVaultIndexer monta o indexador com a configuração de texto
// searchConfig, que o cast para regconfig precisa resolver. O segundo estágio avisa
// em logger o cálculo que parou no teto de iterações.
func NewPostgresVaultIndexer(ctx context.Context, pool *pgxpool.Pool, searchConfig string, logger *slog.Logger) (*PostgresVaultIndexer, error) {
	if _, err := gen.New(pool).ResolveSearchConfig(ctx, searchConfig); err != nil {
		return nil, fmt.Errorf("configuracao de texto %q nao resolvida, esperado nome de pg_ts_config como simple ou pg_catalog.portuguese: %w", searchConfig, err)
	}
	return &PostgresVaultIndexer{pool: pool, searchConfig: searchConfig, logger: logger}, nil
}

// IndexNextVault reserva um cofre com nota pendente ou marcado para o segundo
// estágio, indexa as notas pendentes dele com index e grava o resultado. Depois do
// commit do lote, e já sem a reserva, roda o segundo estágio em transação própria.
// Devolve false quando não há cofre pendente livre.
func (x *PostgresVaultIndexer) IndexNextVault(ctx context.Context, index indexing.IndexFunc) (bool, error) {
	batch, found, err := x.indexNextBatch(ctx, index)
	if !found || err != nil {
		return found, err
	}
	if err := x.analyzeLargeBatch(ctx, batch); err != nil {
		return true, err
	}
	return true, x.secondStage(ctx, batch.vault)
}

// indexedBatch é o lote que uma transação gravou: o cofre, quantas notas vieram e
// quantas o cofre tinha no último df.
type indexedBatch struct {
	vault      gen.ReservePendingVaultRow
	notes      int
	vaultNotes int
}

// indexNextBatch reserva o cofre e grava o lote dele numa transação.
func (x *PostgresVaultIndexer) indexNextBatch(ctx context.Context, index indexing.IndexFunc) (indexedBatch, bool, error) {
	var batch indexedBatch
	found := false
	err := inTx(ctx, x.pool, func(q *gen.Queries) error {
		vault, err := q.ReservePendingVault(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reservar cofre pendente: %w", err)
		}
		found, batch.vault = true, vault
		if batch.notes, err = x.indexVault(ctx, q, vault, index); err != nil {
			return err
		}
		batch.vaultNotes, err = vaultTermNoteCount(ctx, q, vault.VaultID)
		return err
	})
	return batch, found, err
}

// indexVault grava o lote do cofre e devolve quantas notas ele trouxe.
func (x *PostgresVaultIndexer) indexVault(ctx context.Context, q *gen.Queries, vault gen.ReservePendingVaultRow, index indexing.IndexFunc) (int, error) {
	if err := enterVault(ctx, q, vault); err != nil {
		return 0, err
	}
	rows, err := q.ListPendingNotes(ctx, vault.VaultID)
	if err != nil {
		return 0, fmt.Errorf("listar notas pendentes do cofre %q: %w", vault.VaultID, err)
	}
	cs := toChangeset(vault, rows)
	if err := x.applyBatch(ctx, q, cs, index(cs)); err != nil {
		return 0, err
	}
	return len(rows), dequeue(ctx, q, vault.VaultID, rows)
}

// vaultTermNoteCount lê o N do último df do cofre: contar as notas dele custaria O(N)
// a cada lote.
func vaultTermNoteCount(ctx context.Context, q *gen.Queries, vaultID string) (int, error) {
	n, err := q.VaultTermNoteCount(ctx, vaultID)
	if err != nil {
		return 0, fmt.Errorf("ler o N do df do cofre %q: %w", vaultID, err)
	}
	return int(n), nil
}

func enterVault(ctx context.Context, q *gen.Queries, vault gen.ReservePendingVaultRow) error {
	if err := q.EnterIndexer(ctx, gen.EnterIndexerParams{TenantID: vault.TenantID, VaultID: vault.VaultID}); err != nil {
		return fmt.Errorf("entrar no cofre %q do tenant %q: %w", vault.VaultID, vault.TenantID, err)
	}
	return nil
}

// Um lote é grande, e pede ANALYZE das tabelas derivadas antes do segundo estágio,
// quando passa de largeBatchBase notas mais largeBatchFraction das notas do cofre no
// último df: o mesmo limiar padrão do autovacuum (autovacuum_analyze_threshold e
// autovacuum_analyze_scale_factor). O segundo estágio roda logo depois do commit,
// antes de o autovacuum acordar, e sem estatística da carga nova as consultas por
// título planejam como se as tabelas estivessem vazias.
const (
	largeBatchBase     = 50
	largeBatchFraction = 0.1
)

// analyzeDerivedTables analisa as tabelas que o lote grava e o segundo estágio lê.
const analyzeDerivedTables = "ANALYZE note_search_index, note_titles, note_links, note_terms"

// analyzeLargeBatch atualiza as estatísticas das tabelas derivadas quando batch foi
// grande. Roda fora de transação, no papel da conexão, dono das tabelas.
func (x *PostgresVaultIndexer) analyzeLargeBatch(ctx context.Context, batch indexedBatch) error {
	if float64(batch.notes) <= largeBatchBase+largeBatchFraction*float64(batch.vaultNotes) {
		return nil
	}
	if _, err := x.pool.Exec(ctx, analyzeDerivedTables); err != nil {
		return fmt.Errorf("analisar as tabelas derivadas depois do lote de %d notas do cofre %q: %w", batch.notes, batch.vault.VaultID, err)
	}
	return nil
}

// secondStage roda o segundo estágio do cofre numa transação própria, sob a trava
// consultiva do cofre. Outro Job no segundo estágio do mesmo cofre faz este pular:
// a marcação fica, e o próximo Job recalcula.
func (x *PostgresVaultIndexer) secondStage(ctx context.Context, vault gen.ReservePendingVaultRow) error {
	return inTx(ctx, x.pool, func(q *gen.Queries) error {
		locked, err := q.TryLockSecondStage(ctx, vault.VaultID)
		if err != nil {
			return fmt.Errorf("travar o segundo estagio do cofre %q: %w", vault.VaultID, err)
		}
		if !locked {
			return nil
		}
		if err := enterVault(ctx, q, vault); err != nil {
			return err
		}
		return runSecondStage(ctx, q, vault.VaultID, x.searchConfig, x.logger)
	})
}

// applyBatch grava as linhas derivadas do lote: documentos, termos e grafo.
func (x *PostgresVaultIndexer) applyBatch(ctx context.Context, q *gen.Queries, cs indexing.Changeset, indexed []indexing.IndexedNote) error {
	if err := x.saveDocuments(ctx, q, cs, indexed); err != nil {
		return err
	}
	if err := saveTerms(ctx, q, cs, indexed); err != nil {
		return err
	}
	return saveGraphMarkingChange(ctx, q, cs, indexed)
}

func toChangeset(vault gen.ReservePendingVaultRow, rows []gen.ListPendingNotesRow) indexing.Changeset {
	cs := indexing.Changeset{TenantID: vault.TenantID, VaultID: vault.VaultID, Notes: make([]indexing.NoteChange, 0, len(rows))}
	for _, row := range rows {
		cs.Notes = append(cs.Notes, indexing.NoteChange{NoteID: row.ID, Path: row.Path, Content: row.Content})
	}
	return cs
}

// saveDocuments grava os documentos num comando só. Exige um documento por nota do
// lote: nota sem documento ficaria pendente para sempre.
func (x *PostgresVaultIndexer) saveDocuments(ctx context.Context, q *gen.Queries, cs indexing.Changeset, indexed []indexing.IndexedNote) error {
	if err := matchBatch(cs, indexed); err != nil {
		return err
	}
	params := gen.SaveSearchDocumentsParams{SearchConfig: x.searchConfig, NoteIds: make([]string, 0, len(indexed)),
		Titles: make([]string, 0, len(indexed)), Bodies: make([]string, 0, len(indexed))}
	for _, note := range indexed {
		params.NoteIds = append(params.NoteIds, note.Search.NoteID)
		params.Titles = append(params.Titles, strings.Join(append([]string{note.Search.Title}, note.Search.Aliases...), " "))
		params.Bodies = append(params.Bodies, note.Search.Text)
	}
	if err := q.SaveSearchDocuments(ctx, params); err != nil {
		return fmt.Errorf("gravar indice das %d notas do cofre %q: %w", len(indexed), cs.VaultID, err)
	}
	return nil
}

// matchBatch confere que indexed tem exatamente um documento por nota de cs.
func matchBatch(cs indexing.Changeset, indexed []indexing.IndexedNote) error {
	missing := make(map[string]bool, len(cs.Notes))
	for _, note := range cs.Notes {
		missing[note.NoteID] = true
	}
	for _, note := range indexed {
		if !missing[note.Search.NoteID] {
			return fmt.Errorf("documento da nota %q fora do lote do cofre %q, esperado um documento por nota pendente", note.Search.NoteID, cs.VaultID)
		}
		delete(missing, note.Search.NoteID)
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
// no papel da conexão, que precisa ser o dono do banco. Cofre inexistente é erro.
func RequeueNotes(ctx context.Context, pool *pgxpool.Pool, vaultID string) (int64, error) {
	n, err := gen.New(pool).RequeueNotes(ctx, pgtype.Text{String: vaultID, Valid: vaultID != ""})
	if pgErrorCode(err) == noDataFoundCode {
		return 0, fmt.Errorf("cofre %q inexistente, esperado id de um cofre da instalacao ou --all: %w", vaultID, err)
	}
	if err != nil {
		return 0, fmt.Errorf("reenfileirar as notas do cofre %q (vazio para todos), esperado papel dono do banco: %w", vaultID, err)
	}
	return n, nil
}
