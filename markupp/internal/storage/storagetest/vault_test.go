package storagetest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/benchfixture"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

func TestSeedVault_GravaAsNotasNoCofreEEnfileiraTodas(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	require.NoError(t, storage.Migrate(context.Background(), pool))
	ator := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	vault := benchfixture.GenerateVault(benchfixture.VaultSeed, 50)

	ctx := storagetest.SeedVault(t, pool, ator, "va", vault)

	lidas, err := storage.NewPostgresNotesRepository(pool, "simple").ListNotes(ctx)
	require.NoError(t, err)
	assert.Len(t, lidas, 50, "o ator enxerga as notas semeadas pelo contexto devolvido")
	var pendentes int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM note_index_queue").Scan(&pendentes))
	assert.Equal(t, 50, pendentes, "a semeadura passa pelo trigger da fila como uma escrita real")
}
