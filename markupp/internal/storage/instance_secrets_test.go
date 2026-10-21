package storage_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

func segredosDaInstancia(t *testing.T) *storage.PostgresInstanceSecrets {
	t.Helper()
	pool := storagetest.EmptyPool(t)
	_, err := gooseProvider(t, pool).Up(context.Background())
	require.NoError(t, err)
	return storage.NewPostgresInstanceSecrets(pool)
}

func gerador(valor string) func() (string, error) {
	return func() (string, error) { return valor, nil }
}

func TestInstanceSecretGeraNaPrimeiraVezEDevolveOMesmoDepois(t *testing.T) {
	segredos := segredosDaInstancia(t)
	ctx := context.Background()

	primeiro, err := segredos.Secret(ctx, "auth_secret", gerador("gerado-1"))
	require.NoError(t, err)
	segundo, err := segredos.Secret(ctx, "auth_secret", gerador("gerado-2"))
	require.NoError(t, err)

	assert.Equal(t, "gerado-1", primeiro)
	assert.Equal(t, "gerado-1", segundo)
}

func TestInstanceSecretSeparaPorNome(t *testing.T) {
	segredos := segredosDaInstancia(t)
	ctx := context.Background()

	_, err := segredos.Secret(ctx, "auth_secret", gerador("a"))
	require.NoError(t, err)
	outro, err := segredos.Secret(ctx, "mcp_resource_secret", gerador("b"))
	require.NoError(t, err)

	assert.Equal(t, "b", outro)
}

func TestInstanceSecretConcorrenteDevolveUmValorSo(t *testing.T) {
	segredos := segredosDaInstancia(t)
	valores := make([]string, 8)
	var wg sync.WaitGroup
	for i := range valores {
		wg.Go(func() {
			valor, err := segredos.Secret(context.Background(), "auth_secret", gerador(fmt.Sprintf("r%d", i)))
			assert.NoError(t, err)
			valores[i] = valor
		})
	}
	wg.Wait()

	for _, valor := range valores {
		assert.Equal(t, valores[0], valor)
	}
}

func TestInstanceSecretPropagaErroDoGerador(t *testing.T) {
	segredos := segredosDaInstancia(t)
	falha := func() (string, error) { return "", fmt.Errorf("entropia indisponivel") }

	_, err := segredos.Secret(context.Background(), "auth_secret", falha)

	require.ErrorContains(t, err, "entropia indisponivel")
	require.ErrorContains(t, err, "auth_secret")
}

func TestInstanceSecretsFicaForaDosPapeisDaAplicacao(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	_, err := gooseProvider(t, pool).Up(context.Background())
	require.NoError(t, err)

	for _, papel := range []string{"markupp_app", "markupp_scaler", "markupp_indexer", "markupp_notifier"} {
		var permitido bool
		err := pool.QueryRow(context.Background(),
			`SELECT has_table_privilege($1, 'instance_secrets', 'SELECT')`, papel).Scan(&permitido)
		require.NoError(t, err)
		assert.False(t, permitido, "papel %s le instance_secrets", papel)
	}
}
