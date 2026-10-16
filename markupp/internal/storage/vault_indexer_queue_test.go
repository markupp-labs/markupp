package storage_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// textoSemRepeticao passa de 1MB de lexemas distintos, o teto do tsvector.
func textoSemRepeticao() string {
	var b strings.Builder
	for i := range 200_000 {
		fmt.Fprintf(&b, "palavra%d ", i)
	}
	return b.String()
}

func TestVaultIndexer_NotaAcimaDoTetoDoTsvector_IndexaOComecoSemTravarOCofre(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, storage.NewPostgresNotesRepository(db).Save(ctx, notaComConteudo("n1", "va", textoSemRepeticao())))

	found, _ := indexaProximo(t, novoIndexador(t, db, "simple"))

	assert.True(t, found)
	assert.Equal(t, 1, casaNoIndice(t, db, "n1", "simple", "palavra0"))
	assert.Zero(t, countRows(t, db, consultaDoScaler))
}

func TestVaultIndexer_NotaApagadaDuranteAIndexacao_GravaAsOutras(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, repo.Save(ctx, notaComConteudo("n1", "va", "um")))
	require.NoError(t, repo.Save(ctx, notaComConteudo("n2", "va", "dois")))

	found, err := novoIndexador(t, db, "simple").IndexNextVault(context.Background(), func(cs indexing.Changeset) []indexing.SearchDocument {
		require.NoError(t, repo.Delete(ctx, "n1"))
		return indexing.Index(cs)
	})

	require.NoError(t, err, "nota apagada no meio da indexacao nao derruba o lote")
	assert.True(t, found)
	assert.Equal(t, 1, casaNoIndice(t, db, "n2", "simple", "dois"))
	assert.Zero(t, countRows(t, db, consultaDoScaler))
}

func TestVaultIndexer_EscritaDuranteAIndexacao_DeixaANotaPendente(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, repo.Save(ctx, notaComConteudo("n1", "va", "antigo")))
	idx := novoIndexador(t, db, "simple")

	_, err := idx.IndexNextVault(context.Background(), func(cs indexing.Changeset) []indexing.SearchDocument {
		_, err := repo.Update(ctx, "n1", "n1.md", "novo", time.Now(), time.Time{}, true)
		require.NoError(t, err)
		return indexing.Index(cs)
	})

	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, db, consultaDoScaler), "a escrita confirmada durante a indexacao fica para o proximo Job")
	_, lotes := indexaProximo(t, idx)
	require.Len(t, lotes.lotes, 1)
	assert.Equal(t, []indexing.NoteChange{{NoteID: "n1", Content: "novo"}}, lotes.lotes[0].Notes)
}

func TestVaultIndexer_NotaRenomeada_NaoVoltaParaAFila(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, repo.Save(ctx, notaComConteudo("n1", "va", "oi")))
	indexaProximo(t, novoIndexador(t, db, "simple"))

	_, err := repo.Update(ctx, "n1", "outro.md", "oi", time.Now(), time.Time{}, true)

	require.NoError(t, err)
	assert.Zero(t, countRows(t, db, consultaDoScaler), "o indice guarda o conteudo, nao o caminho")
}

func TestVaultIndexer_CofreMudaDeTenant_ReindexaNoTenantNovo(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "ub", TenantID: "t2", Role: "member"}, "vb")
	_, err := db.Exec(ctx, "INSERT INTO notes (id, tenant_id, vault_id, path, content, created_at, updated_at) VALUES ('n1', 't1', 'va', 'n1.md', 'um', now(), now())")
	require.NoError(t, err)
	idx := novoIndexador(t, db, "simple")
	indexaProximo(t, idx)
	_, err = db.Exec(ctx, "UPDATE vaults SET tenant_id = 't2' WHERE id = 'va'")
	require.NoError(t, err, "a adocao de cofres legados troca o tenant do cofre")
	_, err = db.Exec(ctx, "UPDATE notes SET content = 'dois' WHERE id = 'n1'")
	require.NoError(t, err)

	found, lotes := indexaProximo(t, idx)

	require.True(t, found)
	assert.Equal(t, "t2", lotes.lotes[0].TenantID)
	assert.Equal(t, 1, casaNoIndice(t, db, "n1", "simple", "dois"))
}

func TestVaultIndexer_DocumentoDeNotaForaDoLote_RetornaErroComOID(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, storage.NewPostgresNotesRepository(db).Save(ctx, notaComConteudo("n1", "va", "um")))

	_, err := novoIndexador(t, db, "simple").IndexNextVault(context.Background(), func(indexing.Changeset) []indexing.SearchDocument {
		return []indexing.SearchDocument{{NoteID: "n1", Text: "um"}, {NoteID: "intrusa", Text: "x"}}
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "intrusa")
}

func TestVaultIndexer_IndexFuncOmiteNota_RetornaErroComOID(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, storage.NewPostgresNotesRepository(db).Save(ctx, notaComConteudo("n1", "va", "um")))

	_, err := novoIndexador(t, db, "simple").IndexNextVault(context.Background(), func(indexing.Changeset) []indexing.SearchDocument {
		return nil
	})

	require.Error(t, err, "nota sem documento ficaria pendente para sempre")
	assert.Contains(t, err.Error(), "n1")
}

// comoPapel roda sql no papel role, sem tenant nem cofre na sessão.
func comoPapel(t *testing.T, db *pgxpool.Pool, role, sql string) error {
	t.Helper()
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "SELECT set_config('role', $1, true)", role)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, sql)
	return err
}

func TestPendentesDoKEDA_SoOPapelDoScalerExecuta(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, comoPapel(t, db, "markupp_scaler", consultaDoScaler))
	for _, role := range []string{"markupp_app", "markupp_indexer"} {
		err := comoPapel(t, db, role, consultaDoScaler)
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr, role)
		assert.Equal(t, "42501", pgErr.Code, "o papel %s enxerga a contagem de todos os tenants", role)
	}
}

func TestRLS_PapelDoIndexador_SoTiraDaFilaOCofreDaSessao(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ator := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, ator, "va")
	ctx := storagetest.SeedMember(t, db, ator, "vb")
	require.NoError(t, repo.Save(ctx, notaComConteudo("n1", "va", "um")))
	require.NoError(t, repo.Save(ctx, notaComConteudo("n2", "vb", "dois")))

	n, err := comoIndexador(t, db, "t1", "va", "WITH d AS (DELETE FROM note_index_queue RETURNING 1) SELECT count(*) FROM d")

	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "o indexador do cofre va tirou da fila nota de outro cofre")
}

func TestRLS_PapelDoIndexador_NaoGravaIndiceDeOutroCofre(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ator := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, ator, "va")
	ctx := storagetest.SeedMember(t, db, ator, "vb")
	require.NoError(t, repo.Save(ctx, notaComConteudo("n2", "vb", "dois")))

	_, err := comoIndexador(t, db, "t1", "va",
		"WITH i AS (INSERT INTO note_search_index (note_id, document) VALUES ('n2', to_tsvector('x')) RETURNING 1) SELECT count(*) FROM i")

	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "42501", pgErr.Code, "recebido %v", err)
}
