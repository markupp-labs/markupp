package storage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// Hashes de referência calculados fora do banco, com sha256sum sobre os bytes UTF-8.
const (
	sha256DeVazio = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	sha256DeAcao  = "0664077f33cc3ebbaa4bbdacac0eb70e740983080f01dce29929e73b7785a7ad"
)

func TestPostgresRepo_GetNoteByID_DevolveSHA256DoConteudoEmUTF8(t *testing.T) {
	casos := map[string]string{"": sha256DeVazio, "ação": sha256DeAcao}
	for conteudo, esperado := range casos {
		db := setupNotesDB(t)
		repo := storage.NewPostgresNotesRepository(db)
		require.NoError(t, repo.Save(ctxDeTeste, notaComConteudo("n1", cofreDeTeste, conteudo)))

		got, err := repo.GetNoteByID(ctxDeTeste, "n1")

		require.NoError(t, err)
		assert.Equal(t, esperado, got.ContentSHA256, "conteudo %q", conteudo)
	}
}

func TestPostgresRepo_ListNotes_DevolveSHA256DeCadaNota(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	require.NoError(t, repo.Save(ctxDeTeste, notaComConteudo("n1", cofreDeTeste, "ação")))

	got, err := repo.ListNotes(ctxDeTeste)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, sha256DeAcao, got[0].ContentSHA256)
}

func TestPostgresRepo_Update_HashAcompanhaOConteudoNovo(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	require.NoError(t, repo.Save(ctxDeTeste, notaComConteudo("n1", cofreDeTeste, "")))

	got, err := repo.Update(ctxDeTeste, "n1", "n1.md", "ação", time.Now(), time.Time{}, true)

	require.NoError(t, err)
	assert.Equal(t, sha256DeAcao, got.ContentSHA256)
}

func TestVaultIndexer_NotaReescritaComOMesmoConteudo_NaoVoltaParaAFila(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, repo.Save(ctx, notaComConteudo("n1", "va", "ação")))
	indexaProximo(t, novoIndexador(t, db, "simple"))

	_, err := repo.Update(ctx, "n1", "n1.md", "ação", time.Now(), time.Time{}, true)

	require.NoError(t, err)
	assert.Zero(t, countRows(t, db, consultaDoScaler), "mesmo hash nao pede reindexacao")
}

func TestVaultIndexer_ConteudoNovo_VoltaParaAFila(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	require.NoError(t, repo.Save(ctx, notaComConteudo("n1", "va", "ação")))
	indexaProximo(t, novoIndexador(t, db, "simple"))

	_, err := repo.Update(ctx, "n1", "n1.md", "acao", time.Now(), time.Time{}, true)

	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, db, consultaDoScaler), "hash novo pede reindexacao")
}

func TestMigracao_TriggerDaFila_ComparaOHashEmVezDoTexto(t *testing.T) {
	db := setupTestDB(t)

	var condicao string
	require.NoError(t, db.QueryRow(ctxDeTeste,
		"SELECT pg_get_triggerdef(oid) FROM pg_trigger WHERE tgname = 'notes_enqueue_on_update'").Scan(&condicao))

	assert.Contains(t, condicao, "old.content_sha256 IS DISTINCT FROM new.content_sha256")
}
