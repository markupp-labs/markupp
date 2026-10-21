package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// versaoDoGrafo lê, com o papel dono do banco, a versão do grafo que o segundo
// estágio gravou para o cofre vaultID.
func versaoDoGrafo(t *testing.T, db *pgxpool.Pool, vaultID string) int64 {
	t.Helper()
	var v int64
	require.NoError(t, db.QueryRow(context.Background(), "SELECT version FROM vault_graph_versions WHERE vault_id = $1", vaultID).Scan(&v), "versao do grafo do cofre %q", vaultID)
	return v
}

func TestVersaoDoGrafo_SegundoEstagioGrava(t *testing.T) {
	db, _ := cofreComDf(t)

	assert.Positive(t, versaoDoGrafo(t, db, cofreDeTeste))
}

func TestVersaoDoGrafo_MudaSoQuandoOSegundoEstagioRoda(t *testing.T) {
	db, repo := cofreComDf(t)
	antes := versaoDoGrafo(t, db, cofreDeTeste)

	_, err := repo.Update(ctxDeTeste, "n1", "n1.md", "alfa beta delta", time.Now(), time.Time{}, true)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")
	require.Equal(t, antes, versaoDoGrafo(t, db, cofreDeTeste), "nem ligacao nem titulo mudou")

	_, err = repo.Update(ctxDeTeste, "n1", "n1.md", "alfa [[n2]]", time.Now(), time.Time{}, true)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")
	assert.Greater(t, versaoDoGrafo(t, db, cofreDeTeste), antes)
}

func TestVersaoDoGrafo_SaiComOCofre(t *testing.T) {
	db, _ := cofreComDf(t)

	_, err := db.Exec(context.Background(), "DELETE FROM vaults WHERE id = $1", cofreDeTeste)

	require.NoError(t, err)
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM vault_graph_versions"))
}

func TestRLS_PapelDoIndexador_NaoGravaVersaoDoGrafoDeOutroCofre(t *testing.T) {
	db := setupTestDB(t)
	ator := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, ator, "va")
	storagetest.SeedMember(t, db, ator, "vb")

	_, err := comoIndexador(t, db, "t1", "va", "WITH i AS (INSERT INTO vault_graph_versions (vault_id, version) VALUES ('vb', 1) RETURNING 1) SELECT count(*) FROM i")

	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "42501", pgErr.Code, "recebido %v", err)
}
