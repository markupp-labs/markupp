package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// doisTenantsIndexados semeia o cofre va no tenant t1 e o vb no t2, com duas e uma
// notas, e esvazia a fila indexando os dois.
func doisTenantsIndexados(t *testing.T) *pgxpool.Pool {
	t.Helper()
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctxA := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	ctxB := storagetest.SeedMember(t, db, identity.Actor{UserID: "ub", TenantID: "t2", Role: "member"}, "vb")
	require.NoError(t, repo.Save(ctxA, notaComConteudo("a1", "va", "um")))
	require.NoError(t, repo.Save(ctxA, notaComConteudo("a2", "va", "dois")))
	require.NoError(t, repo.Save(ctxB, notaComConteudo("b1", "vb", "tres")))
	indexaTodosOsCofres(t, db, "simple")
	require.Zero(t, countRows(t, db, consultaDoScaler))
	return db
}

func pendentesDoCofre(t *testing.T, db *pgxpool.Pool, vaultID string) int {
	t.Helper()
	var n int
	err := db.QueryRow(context.Background(),
		"SELECT count(*) FROM note_index_queue q JOIN notes n ON n.id = q.note_id WHERE n.vault_id = $1", vaultID).Scan(&n)
	require.NoError(t, err)
	return n
}

func TestRequeueNotes_UmCofre_ReenfileiraSoAsNotasDele(t *testing.T) {
	db := doisTenantsIndexados(t)

	n, err := storage.RequeueNotes(context.Background(), db, "va")

	require.NoError(t, err)
	assert.EqualValues(t, 2, n)
	assert.Equal(t, 2, pendentesDoCofre(t, db, "va"))
	assert.Zero(t, pendentesDoCofre(t, db, "vb"), "o cofre do outro tenant fica fora")
}

func TestRequeueNotes_SemCofre_ReenfileiraTodosOsCofresDaInstalacao(t *testing.T) {
	db := doisTenantsIndexados(t)

	n, err := storage.RequeueNotes(context.Background(), db, "")

	require.NoError(t, err)
	assert.EqualValues(t, 3, n)
	assert.Equal(t, 2, countRows(t, db, consultaDoScaler))
}

func TestRequeueNotes_NotaJaNaFila_GanhaRevisaoNova(t *testing.T) {
	db := doisTenantsIndexados(t)
	_, err := storage.RequeueNotes(context.Background(), db, "vb")
	require.NoError(t, err)
	antes := countRows(t, db, "SELECT revision FROM note_index_queue WHERE note_id = 'b1'")

	_, err = storage.RequeueNotes(context.Background(), db, "vb")

	require.NoError(t, err)
	assert.Greater(t, countRows(t, db, "SELECT revision FROM note_index_queue WHERE note_id = 'b1'"), antes,
		"o indexador que leu a revisao antiga nao tira a nota da fila")
}

func TestRequeueNotes_PapeisDeTenantEDoKEDA_NaoExecutam(t *testing.T) {
	db := doisTenantsIndexados(t)

	for _, role := range []string{"markupp_app", "markupp_indexer", "markupp_scaler"} {
		err := comoPapel(t, db, role, "SELECT markupp_requeue_notes(NULL)")
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr, role)
		assert.Equal(t, "42501", pgErr.Code, "o papel %s reenfileiraria notas de todos os tenants", role)
	}
}
