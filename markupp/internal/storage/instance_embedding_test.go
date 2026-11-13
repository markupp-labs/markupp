package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

func bancoDaInstancia(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := storagetest.EmptyPool(t)
	_, err := gooseProvider(t, pool).Up(context.Background())
	require.NoError(t, err)
	return pool
}

// comAtor é o contexto da API: a escolha é gravada no papel markupp_app.
func comAtor() context.Context {
	return identity.WithActor(context.Background(), identity.Actor{UserID: "u-admin"})
}

func TestInstanceEmbedding_BancoNovo_SemProvedorSemTravaSemEstado(t *testing.T) {
	repo := storage.NewAppInstanceEmbedding(bancoDaInstancia(t))

	record, err := repo.LoadEmbedding(comAtor())

	require.NoError(t, err)
	assert.Equal(t, "none", record.Choice.Provider)
	assert.False(t, record.HostLocked)
	assert.Equal(t, instance.ModelStatus{}, record.Model)
}

func TestInstanceEmbedding_APIGravaAEscolha_IndexadorLe(t *testing.T) {
	pool := bancoDaInstancia(t)
	app := storage.NewAppInstanceEmbedding(pool)
	antes, err := app.LoadEmbedding(comAtor())
	require.NoError(t, err)
	escolha := instance.EmbeddingChoice{Provider: "openai-compat", URL: "https://api.exemplo/v1", Model: "m", SealedAPIKey: "selado"}

	require.NoError(t, app.SaveEmbeddingChoice(comAtor(), escolha))
	record, err := storage.NewIndexerInstanceEmbedding(pool).LoadEmbedding(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "selado", record.Choice.SealedAPIKey)
	assert.Equal(t, "https://api.exemplo/v1", record.Choice.URL)
	assert.True(t, record.Choice.UpdatedAt.After(antes.Choice.UpdatedAt), "cada gravação muda a revisão")
}

func TestInstanceEmbedding_IndexadorInformaTravaEEstado_APILe(t *testing.T) {
	pool := bancoDaInstancia(t)
	indexer := storage.NewIndexerInstanceEmbedding(pool)

	require.NoError(t, indexer.ReportEmbeddingHost(context.Background(), true, "local"))
	require.NoError(t, indexer.ReportModelStatus(context.Background(), instance.ModelError, "sem rede"))
	record, err := storage.NewAppInstanceEmbedding(pool).LoadEmbedding(comAtor())

	require.NoError(t, err)
	assert.True(t, record.HostLocked)
	assert.Equal(t, "local", record.HostProvider)
	assert.Equal(t, instance.ModelError, record.Model.State)
	assert.Equal(t, "sem rede", record.Model.Error)
	assert.NotNil(t, record.Model.ChangedAt)
}

func TestInstanceEmbedding_PapelDoIndexador_NaoGravaAEscolha(t *testing.T) {
	err := storage.NewIndexerInstanceEmbedding(bancoDaInstancia(t)).SaveEmbeddingChoice(context.Background(), instance.EmbeddingChoice{Provider: "local"})

	assert.Error(t, err)
}

func TestInstanceEmbedding_PapelDaAPI_NaoEscreveOEstadoDoModelo(t *testing.T) {
	pool := bancoDaInstancia(t)
	ctx := comAtor()

	err := storage.NewAppInstanceEmbedding(pool).ReportModelStatus(ctx, instance.ModelReady, "")

	assert.Error(t, err)
}

func TestInstanceEmbedding_PapelDaAPI_NaoLeOsSegredosDaInstancia(t *testing.T) {
	pool := bancoDaInstancia(t)
	tx, err := pool.Begin(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	_, err = tx.Exec(context.Background(), "SET LOCAL ROLE markupp_app")
	require.NoError(t, err)

	_, err = tx.Exec(context.Background(), "SELECT * FROM instance_secrets")

	assert.Error(t, err, "a chave de cifra do banco fica fora do papel da aplicação")
}
