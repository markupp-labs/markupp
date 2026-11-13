package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

func TestParseCommand_EmbedLoop_RetornaIndexacaoDeEmbeddingEmLaco(t *testing.T) {
	cmd, err := parseCommand([]string{"markupp", "embed-loop"})

	require.NoError(t, err)
	assert.Equal(t, embedLoopCommand, cmd)
}

func TestRun_EmbedLoopLocalSemONNXRuntime_RetornaErroSemAbrirOBanco(t *testing.T) {
	t.Setenv("MARKUPP_DATABASE_URL", "postgres://ninguem@127.0.0.1:1/nada?connect_timeout=1")
	t.Setenv("MARKUPP_EMBEDDING_PROVIDER", "local")

	err := run(context.Background(), slog.New(slog.DiscardHandler), []string{"markupp", "embed-loop"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "markupp-embedder")
}

func TestRun_EmbedLoopSemProvedor_DesligaOModeloEFicaOcioso(t *testing.T) {
	url := storagetest.DatabaseURL(t)
	require.NoError(t, migrateDatabase(context.Background(), url, ""))
	pool, err := storage.OpenPool(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, err = storage.UseEmbeddingModel(context.Background(), pool, "anterior", 2)
	require.NoError(t, err)
	t.Setenv("MARKUPP_DATABASE_URL", url)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- run(ctx, slog.New(slog.DiscardHandler), []string{"markupp", "embed-loop"}) }()

	assert.Eventually(t, func() bool {
		var nome string
		return pool.QueryRow(context.Background(), "SELECT name FROM embedding_model").Scan(&nome) == nil && nome == "none"
	}, 10*time.Second, 20*time.Millisecond, "sem provedor, os vetores do modelo anterior saem")
	cancel()
	require.NoError(t, <-done)
}
