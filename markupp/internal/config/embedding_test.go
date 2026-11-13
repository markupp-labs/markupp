package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
)

func TestLoadEmbedding_SemVariaveis_SemProvedorEPedacosDe512(t *testing.T) {
	cfg, err := config.LoadEmbedding(fakeEnv{}.Lookup)

	require.NoError(t, err)
	assert.Equal(t, config.EmbeddingNone, cfg.Provider)
	assert.Equal(t, 512, cfg.ChunkTokens)
	assert.Equal(t, "/var/lib/markupp/models", cfg.ModelDir)
}

func TestLoadEmbedding_Remoto_LeEnderecoModeloEChave(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_EMBEDDING_PROVIDER": "openai-compat", "MARKUPP_EMBEDDING_URL": "https://api.exemplo/v1",
		"MARKUPP_EMBEDDING_MODEL": "modelo-x", "MARKUPP_EMBEDDING_API_KEY": "chave", "MARKUPP_EMBEDDING_CHUNK_TOKENS": "256",
	}

	cfg, err := config.LoadEmbedding(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, config.Embedding{Provider: config.EmbeddingOpenAICompat, URL: "https://api.exemplo/v1", Model: "modelo-x",
		APIKey: "chave", ModelDir: "/var/lib/markupp/models", ChunkTokens: 256}, cfg)
}

func TestLoadEmbedding_RemotoSemEndereco_RetornaErroComAVariavel(t *testing.T) {
	_, err := config.LoadEmbedding(fakeEnv{"MARKUPP_EMBEDDING_PROVIDER": "openai-compat", "MARKUPP_EMBEDDING_MODEL": "modelo-x"}.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_EMBEDDING_URL")
}

func TestLoadEmbedding_RemotoSemModelo_RetornaErroComAVariavel(t *testing.T) {
	_, err := config.LoadEmbedding(fakeEnv{"MARKUPP_EMBEDDING_PROVIDER": "openai-compat", "MARKUPP_EMBEDDING_URL": "https://api.exemplo/v1"}.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_EMBEDDING_MODEL")
}

func TestLoadEmbedding_ProvedorDesconhecido_RetornaErroComOsValoresAceitos(t *testing.T) {
	_, err := config.LoadEmbedding(fakeEnv{"MARKUPP_EMBEDDING_PROVIDER": "ollama"}.Lookup)

	require.Error(t, err)
	for _, esperado := range []string{"ollama", "none", "openai-compat", "local"} {
		assert.Contains(t, err.Error(), esperado)
	}
}

func TestLoadEmbedding_PedacoForaDaFaixa_RetornaErroComOValor(t *testing.T) {
	_, err := config.LoadEmbedding(fakeEnv{"MARKUPP_EMBEDDING_PROVIDER": "local", "MARKUPP_EMBEDDING_CHUNK_TOKENS": "4"}.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_EMBEDDING_CHUNK_TOKENS")
}

func TestLoadEmbedding_Local_LeAPastaDoModelo(t *testing.T) {
	cfg, err := config.LoadEmbedding(fakeEnv{"MARKUPP_EMBEDDING_PROVIDER": "local", "MARKUPP_EMBEDDING_MODEL_DIR": "/modelos"}.Lookup)

	require.NoError(t, err)
	assert.Equal(t, config.EmbeddingLocal, cfg.Provider)
	assert.Equal(t, "/modelos", cfg.ModelDir)
}
