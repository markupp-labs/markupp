package storage_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

func novoIndexador(t *testing.T, db *pgxpool.Pool, searchConfig string) *storage.PostgresVaultIndexer {
	t.Helper()
	idx, err := storage.NewPostgresVaultIndexer(context.Background(), db, searchConfig)
	require.NoError(t, err)
	return idx
}

// consultaDoScaler é a consulta que o ScaledJob do KEDA faz ao banco.
const consultaDoScaler = "SELECT markupp_pending_vaults()"

func notaComConteudo(id, vaultID, content string) notes.Note {
	agora := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	return notes.Note{ID: id, VaultID: vaultID, Path: id + ".md", Content: content, CreatedAt: agora, UpdatedAt: agora}
}

// lotesRecebidos guarda cada lote que o storage entrega à função de indexação e
// indexa com indexing.Index, como o binário faz.
type lotesRecebidos struct {
	mu    sync.Mutex
	lotes []indexing.Changeset
}

func (l *lotesRecebidos) index(cs indexing.Changeset) []indexing.SearchDocument {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lotes = append(l.lotes, cs)
	return indexing.Index(cs)
}

func indexaProximo(t *testing.T, idx *storage.PostgresVaultIndexer) (bool, *lotesRecebidos) {
	t.Helper()
	lotes := &lotesRecebidos{}
	found, err := idx.IndexNextVault(context.Background(), lotes.index)
	require.NoError(t, err)
	return found, lotes
}

func casaNoIndice(t *testing.T, db *pgxpool.Pool, noteID, config, termo string) int {
	t.Helper()
	var n int
	err := db.QueryRow(context.Background(),
		"SELECT count(*) FROM note_search_index WHERE note_id = $1 AND document @@ to_tsquery($2::regconfig, $3)",
		noteID, config, termo).Scan(&n)
	require.NoError(t, err)
	return n
}

func TestVaultIndexer_SemCofrePendente_DevolveFalsoSemIndexar(t *testing.T) {
	db := setupTestDB(t)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")

	found, lotes := indexaProximo(t, novoIndexador(t, db, "simple"))

	assert.False(t, found)
	assert.Empty(t, lotes.lotes)
}

func TestVaultIndexer_NotaNova_IndexaOConteudoNoCofreDela(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, storage.NewPostgresNotesRepository(db).Save(ctx, notaComConteudo("n1", "va", "reuniao com o cliente")))

	found, lotes := indexaProximo(t, novoIndexador(t, db, "simple"))

	require.True(t, found)
	require.Len(t, lotes.lotes, 1)
	assert.Equal(t, indexing.Changeset{TenantID: "t1", VaultID: "va", Notes: []indexing.NoteChange{
		{NoteID: "n1", Content: "reuniao com o cliente"},
	}}, lotes.lotes[0])
	assert.Equal(t, 1, casaNoIndice(t, db, "n1", "simple", "cliente"))
}

func TestVaultIndexer_CofreJaIndexado_NaoFicaPendente(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, storage.NewPostgresNotesRepository(db).Save(ctx, notaComConteudo("n1", "va", "oi")))
	idx := novoIndexador(t, db, "simple")
	indexaProximo(t, idx)

	found, lotes := indexaProximo(t, idx)

	assert.False(t, found, "a fila do cofre esvazia na mesma transação da indexação")
	assert.Empty(t, lotes.lotes)
}

func TestVaultIndexer_NotaEditada_ReindexaSoElaComOConteudoNovo(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, repo.Save(ctx, notaComConteudo("n1", "va", "palavra antiga")))
	require.NoError(t, repo.Save(ctx, notaComConteudo("n2", "va", "intocada")))
	idx := novoIndexador(t, db, "simple")
	indexaProximo(t, idx)
	_, err := repo.Update(ctx, "n1", "n1.md", "palavra nova", time.Now(), time.Time{}, true)
	require.NoError(t, err)

	found, lotes := indexaProximo(t, idx)

	require.True(t, found)
	require.Len(t, lotes.lotes, 1)
	assert.Equal(t, []indexing.NoteChange{{NoteID: "n1", Content: "palavra nova"}}, lotes.lotes[0].Notes,
		"so a nota que mudou desde a ultima indexacao entra no lote")
	assert.Equal(t, 1, casaNoIndice(t, db, "n1", "simple", "nova"))
	assert.Zero(t, casaNoIndice(t, db, "n1", "simple", "antiga"))
	assert.Equal(t, 1, casaNoIndice(t, db, "n2", "simple", "intocada"))
}

func TestVaultIndexer_NotaApagada_SaiDoIndice(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, repo.Save(ctx, notaComConteudo("n1", "va", "oi")))
	indexaProximo(t, novoIndexador(t, db, "simple"))

	require.NoError(t, repo.Delete(ctx, "n1"))

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM note_search_index WHERE note_id = 'n1'"))
}

func TestVaultIndexer_LoteSoTrazNotasDoCofreReservado(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ator := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, ator, "va")
	ctx := storagetest.SeedMember(t, db, ator, "vb")
	require.NoError(t, repo.Save(ctx, notaComConteudo("n1", "va", "um")))
	require.NoError(t, repo.Save(ctx, notaComConteudo("n2", "vb", "dois")))

	_, lotes := indexaProximo(t, novoIndexador(t, db, "simple"))

	require.Len(t, lotes.lotes, 1)
	require.Len(t, lotes.lotes[0].Notes, 1, "um Job indexa um cofre só")
}

// reservaPresa indexa o próximo cofre numa goroutine e segura a transação aberta
// dentro da função de indexação até libera ser chamada. O cleanup libera a
// transação mesmo quando um require aborta o teste antes.
func reservaPresa(t *testing.T, idx *storage.PostgresVaultIndexer) (reservado indexing.Changeset, libera func(), terminou <-chan error) {
	t.Helper()
	cofre := make(chan indexing.Changeset, 1)
	solta := make(chan struct{})
	libera = sync.OnceFunc(func() { close(solta) })
	t.Cleanup(libera)
	fim := make(chan error, 1)
	go func() {
		_, err := idx.IndexNextVault(context.Background(), func(cs indexing.Changeset) []indexing.SearchDocument {
			cofre <- cs
			<-solta
			return indexing.Index(cs)
		})
		fim <- err
	}()
	select {
	case reservado = <-cofre:
	case err := <-fim:
		require.FailNow(t, "a indexacao terminou sem reservar cofre", "erro: %v", err)
	case <-time.After(10 * time.Second):
		require.FailNow(t, "a indexacao nao reservou cofre em 10s")
	}
	return reservado, libera, fim
}

func TestVaultIndexer_CofreReservado_OutroIndexadorPulaParaOProximo(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctxA := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	ctxB := storagetest.SeedMember(t, db, identity.Actor{UserID: "ub", TenantID: "t2", Role: "member"}, "vb")
	require.NoError(t, repo.Save(ctxA, notaComConteudo("n1", "va", "um")))
	require.NoError(t, repo.Save(ctxB, notaComConteudo("n2", "vb", "dois")))
	idx := novoIndexador(t, db, "simple")
	primeiro, libera, terminou := reservaPresa(t, idx)

	found, lotes := indexaProximo(t, idx)
	require.True(t, found)
	require.Len(t, lotes.lotes, 1)
	assert.NotEqual(t, primeiro.VaultID, lotes.lotes[0].VaultID, "dois Jobs nunca indexam o mesmo cofre")
	found, _ = indexaProximo(t, idx)
	assert.False(t, found, "o único cofre pendente que sobra está reservado")

	libera()
	require.NoError(t, <-terminou)
}

func TestVaultIndexer_PendentesDoKEDA_ContaCofresComMudanca(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctxA := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	ctxB := storagetest.SeedMember(t, db, identity.Actor{UserID: "ub", TenantID: "t2", Role: "member"}, "vb")
	assert.Zero(t, countRows(t, db, consultaDoScaler))

	require.NoError(t, repo.Save(ctxA, notaComConteudo("n1", "va", "um")))
	require.NoError(t, repo.Save(ctxB, notaComConteudo("n2", "vb", "dois")))
	assert.Equal(t, 2, countRows(t, db, consultaDoScaler))

	indexaProximo(t, novoIndexador(t, db, "simple"))
	assert.Equal(t, 1, countRows(t, db, consultaDoScaler))
}

func TestNewPostgresVaultIndexer_ConfiguracaoInexistente_RetornaErroComOValor(t *testing.T) {
	_, err := storage.NewPostgresVaultIndexer(context.Background(), setupTestDB(t), "klingon")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "klingon")
	assert.Contains(t, err.Error(), "simple")
}

func TestNewPostgresVaultIndexer_ConfiguracaoComSchema_Aceita(t *testing.T) {
	_, err := storage.NewPostgresVaultIndexer(context.Background(), setupTestDB(t), "pg_catalog.portuguese")

	assert.NoError(t, err, "a validacao resolve o nome como o cast para regconfig resolve")
}

func TestVaultIndexer_ConfiguracaoEnglish_IndexaPorRadical(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, storage.NewPostgresNotesRepository(db).Save(ctx, notaComConteudo("n1", "va", "running tests")))

	indexaProximo(t, novoIndexador(t, db, "english"))

	assert.Equal(t, 1, casaNoIndice(t, db, "n1", "english", "run"))
}

// comoIndexador roda sql no papel markupp_indexer, preso ao tenant e ao cofre da
// sessão, sem passar pelo storage: prova a política do banco.
func comoIndexador(t *testing.T, db *pgxpool.Pool, tenantID, vaultID, sql string) (int64, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "SELECT set_config('role', 'markupp_indexer', true), set_config('markupp.tenant_id', $1, true), set_config('markupp.vault_id', $2, true)",
		tenantID, vaultID)
	require.NoError(t, err)
	var n int64
	err = tx.QueryRow(ctx, sql).Scan(&n)
	return n, err
}

func TestRLS_PapelDoIndexador_SoEnxergaOCofreDaSessao(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ator := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, ator, "va")
	ctx := storagetest.SeedMember(t, db, ator, "vb")
	require.NoError(t, repo.Save(ctx, notaComConteudo("n1", "va", "um")))
	require.NoError(t, repo.Save(ctx, notaComConteudo("n2", "vb", "dois")))

	n, err := comoIndexador(t, db, "t1", "va", "SELECT count(*) FROM notes")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "o indexador do cofre va enxergou nota de outro cofre")
	n, err = comoIndexador(t, db, "t2", "va", "SELECT count(*) FROM notes")
	require.NoError(t, err)
	assert.Zero(t, n, "cofre de outro tenant na sessão não abre as notas")
}

func TestRLS_PapelDoIndexador_NaoEscreveNoRegistro(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, storage.NewPostgresNotesRepository(db).Save(ctx, notaComConteudo("n1", "va", "um")))

	_, err := comoIndexador(t, db, "t1", "va", "WITH u AS (UPDATE notes SET content = 'x' RETURNING 1) SELECT count(*) FROM u")

	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "42501", pgErr.Code, "o plano de recuperação nunca escreve no registro (ADR-0012), recebido %v", err)
}
