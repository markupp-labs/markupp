package main

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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

// bancoDoEmbedder migra um banco novo e aponta o embed-loop para ele, com intervalo curto.
func bancoDoEmbedder(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := storagetest.DatabaseURL(t)
	require.NoError(t, migrateDatabase(context.Background(), url, ""))
	pool, err := storage.OpenPool(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	t.Setenv("MARKUPP_DATABASE_URL", url)
	t.Setenv("MARKUPP_INDEX_INTERVAL", "20ms")
	return pool
}

// rodarEmbedder sobe o embed-loop em segundo plano e o encerra no fim do teste.
func rodarEmbedder(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, slog.New(slog.DiscardHandler), []string{"markupp", "embed-loop"}) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
}

// estadoDoModelo lê a trava e o estado do modelo que o embed-loop informou.
func estadoDoModelo(pool *pgxpool.Pool) (locked bool, hostProvider, state, message string) {
	_ = pool.QueryRow(context.Background(), "SELECT host_locked, host_provider, model_state, model_error FROM instance_embedding").
		Scan(&locked, &hostProvider, &state, &message)
	return locked, hostProvider, state, message
}

func TestRun_EmbedLoopLocalNaVariavelSemONNXRuntime_InformaTravaEErro(t *testing.T) {
	pool := bancoDoEmbedder(t)
	t.Setenv("MARKUPP_EMBEDDING_PROVIDER", "local")

	rodarEmbedder(t)

	assert.Eventually(t, func() bool {
		locked, host, state, message := estadoDoModelo(pool)
		return locked && host == "local" && state == "error" && strings.Contains(message, "markupp-embedder")
	}, 10*time.Second, 20*time.Millisecond)
}

func TestRun_EmbedLoopSemVariavel_TrocaPelaEscolhaDoBancoSemReiniciar(t *testing.T) {
	pool := bancoDoEmbedder(t)
	rodarEmbedder(t)
	require.Eventually(t, func() bool {
		var nome string
		return pool.QueryRow(context.Background(), "SELECT name FROM embedding_model").Scan(&nome) == nil && nome == "none"
	}, 10*time.Second, 20*time.Millisecond)

	_, err := pool.Exec(context.Background(), "UPDATE instance_embedding SET provider = 'local', updated_at = clock_timestamp()")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		locked, _, state, message := estadoDoModelo(pool)
		return !locked && state == "error" && strings.Contains(message, "markupp-embedder")
	}, 10*time.Second, 20*time.Millisecond)
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
