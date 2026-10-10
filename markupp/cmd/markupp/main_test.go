package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

func TestNewServer_PortaConfigurada_DefineEnderecoETimeouts(t *testing.T) {
	server := newServer(8080, http.NotFoundHandler())

	assert.Equal(t, ":8080", server.Addr)
	assert.Greater(t, server.ReadHeaderTimeout, time.Duration(0),
		"ReadHeaderTimeout zerado deixa o servidor exposto a slowloris")
	assert.Greater(t, server.ReadTimeout, time.Duration(0))
	assert.Greater(t, server.WriteTimeout, time.Duration(0))
	assert.Greater(t, server.IdleTimeout, time.Duration(0))
}

func TestParseCommand_SemArgumento_RetornaServe(t *testing.T) {
	cmd, err := parseCommand([]string{"markupp"})

	require.NoError(t, err)
	assert.Equal(t, serveCommand, cmd)
}

func TestParseCommand_Migrate_RetornaMigrate(t *testing.T) {
	cmd, err := parseCommand([]string{"markupp", "migrate"})

	require.NoError(t, err)
	assert.Equal(t, migrateCommand, cmd)
}

func TestParseCommand_Desconhecido_RetornaErroComOValor(t *testing.T) {
	_, err := parseCommand([]string{"markupp", "migrar"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "migrar")
	assert.Contains(t, err.Error(), "migrate")
}

func TestMigrateDatabase_BancoVazio_CriaTabelaNotes(t *testing.T) {
	url := storagetest.DatabaseURL(t)

	require.NoError(t, migrateDatabase(context.Background(), url))

	pool, err := storage.OpenPool(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	var tabelaNotes, tabelaUsers *string
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT to_regclass('notes')::text").Scan(&tabelaNotes))
	assert.NotNil(t, tabelaNotes, "esperada a tabela notes depois do migrate")
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT to_regclass('users')::text").Scan(&tabelaUsers))
	assert.NotNil(t, tabelaUsers, "esperada a tabela users depois do migrate")
}

func TestMigrateDatabase_BancoInalcancavel_RetornaErro(t *testing.T) {
	err := migrateDatabase(context.Background(), "postgres://ninguem@127.0.0.1:1/nada?connect_timeout=1")

	assert.Error(t, err)
}
