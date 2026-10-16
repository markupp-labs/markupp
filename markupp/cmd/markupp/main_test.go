package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
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

	require.NoError(t, migrateDatabase(context.Background(), url, ""))

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
	err := migrateDatabase(context.Background(), "postgres://ninguem@127.0.0.1:1/nada?connect_timeout=1", "")

	assert.Error(t, err)
}

func TestParseCommand_Index_RetornaIndexacaoUnica(t *testing.T) {
	cmd, err := parseCommand([]string{"markupp", "index"})

	require.NoError(t, err)
	assert.Equal(t, indexCommand, cmd)
}

func TestParseCommand_IndexLoop_RetornaIndexacaoEmLaco(t *testing.T) {
	cmd, err := parseCommand([]string{"markupp", "index-loop"})

	require.NoError(t, err)
	assert.Equal(t, indexLoopCommand, cmd)
}

func TestParseCommand_Desconhecido_ListaTodosOsSubcomandos(t *testing.T) {
	_, err := parseCommand([]string{"markupp", "indexar"})

	require.Error(t, err)
	for _, esperado := range []string{"indexar", "serve", "migrate", "index", "index-loop"} {
		assert.Contains(t, err.Error(), esperado)
	}
}

func TestRun_Index_IndexaOCofrePendenteESai(t *testing.T) {
	url := storagetest.DatabaseURL(t)
	require.NoError(t, migrateDatabase(context.Background(), url, ""))
	pool, err := storage.OpenPool(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	ctx := storagetest.SeedMember(t, pool, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	agora := time.Now()
	require.NoError(t, storage.NewPostgresNotesRepository(pool).Save(ctx,
		notes.Note{ID: "n1", VaultID: "va", Path: "a.md", Content: "oi", CreatedAt: agora, UpdatedAt: agora}))
	t.Setenv("MARKUPP_DATABASE_URL", url)
	t.Setenv("MARKUPP_AUTH_SECRET", "")

	require.NoError(t, run(context.Background(), slog.New(slog.DiscardHandler), []string{"markupp", "index"}),
		"o Job do indexador nao recebe o segredo dos JWT")

	var pendentes int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT markupp_pending_vaults()").Scan(&pendentes))
	assert.Zero(t, pendentes)
}

func TestMigrateDatabase_ComSenhaDoScaler_LiberaOLoginDoPapel(t *testing.T) {
	dono := storagetest.DatabaseURL(t)

	require.NoError(t, migrateDatabase(context.Background(), dono, "senha-do-scaler"))

	scaler, err := url.Parse(dono)
	require.NoError(t, err)
	scaler.User = url.UserPassword("markupp_scaler", "senha-do-scaler")
	conn, err := pgx.Connect(context.Background(), scaler.String())
	require.NoError(t, err, "o KEDA entra com o papel do scaler, nao com o dono do banco")
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	var pendentes int
	require.NoError(t, conn.QueryRow(context.Background(), "SELECT markupp_pending_vaults()").Scan(&pendentes))
	assert.Zero(t, pendentes)
}
