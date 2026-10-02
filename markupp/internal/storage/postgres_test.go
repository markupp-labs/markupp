package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

const urlSemServidor = "postgres://ninguem@127.0.0.1:1/nada?sslmode=disable&connect_timeout=1"

func TestOpenPool_BancoDisponivel_RetornaPoolRespondendoAoPing(t *testing.T) {
	pool, err := storage.OpenPool(context.Background(), storagetest.DatabaseURL(t))

	require.NoError(t, err)
	require.NotNil(t, pool)
	t.Cleanup(pool.Close)
	assert.NoError(t, pool.Ping(context.Background()))
}

func TestOpenPool_ServidorInalcancavel_RetornaErroENilPool(t *testing.T) {
	pool, err := storage.OpenPool(context.Background(), urlSemServidor)

	require.Error(t, err, "esperado erro de conexao para %q", urlSemServidor)
	assert.Nil(t, pool)
}

func TestOpenPool_URLMalformada_RetornaErroENilPool(t *testing.T) {
	pool, err := storage.OpenPool(context.Background(), "::nao-e-url")

	require.Error(t, err)
	assert.Nil(t, pool)
}

func TestMigrate_BancoVazio_CriaTabelaNotes(t *testing.T) {
	pool := storagetest.EmptyPool(t)

	require.NoError(t, storage.Migrate(context.Background(), pool))

	var tabela *string
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT to_regclass('notes')::text").Scan(&tabela))
	require.NotNil(t, tabela, "esperada a tabela notes depois da migracao")
	assert.Equal(t, "notes", *tabela)
}

func TestMigrate_AplicadaDuasVezes_NaoFalha(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	require.NoError(t, storage.Migrate(context.Background(), pool))

	assert.NoError(t, storage.Migrate(context.Background(), pool),
		"o job de migracao roda a cada versao e precisa ser idempotente")
}

func TestMigrate_PoolFechado_RetornaErro(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	pool.Close()

	err := storage.Migrate(context.Background(), pool)

	assert.Error(t, err, "esperado erro ao migrar sobre pool fechado")
}
