package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
)

const urlDeTeste = "postgres://markupp@db:5432/markupp"

// fakeEnv faz o papel das variáveis de ambiente do processo, com a mesma
// assinatura de os.LookupEnv.
type fakeEnv map[string]string

func (f fakeEnv) Lookup(key string) (string, bool) {
	value, ok := f[key]
	return value, ok
}

func TestDefault_RetornaValoresPadrao(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, int64(50*1024*1024), cfg.MaxNoteSize)
	assert.Empty(t, cfg.DatabaseURL, "sem default: o banco precisa ser informado")
	assert.Empty(t, cfg.AllowedOrigins, "sem origem configurada o CORS fica desligado")
}

func TestLoad_SemDatabaseURL_RetornaErroNomeandoAVariavel(t *testing.T) {
	_, err := config.Load(fakeEnv{}.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_DATABASE_URL")
}

func TestLoad_SoDatabaseURL_UsaDefaultsNoResto(t *testing.T) {
	cfg, err := config.Load(fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste}.Lookup)

	require.NoError(t, err)
	esperado := config.Default()
	esperado.DatabaseURL = urlDeTeste
	assert.Equal(t, esperado, cfg)
}

func TestLoad_TodasAsVariaveis_LeTodosOsCampos(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL":    urlDeTeste,
		"MARKUPP_PORT":            "9090",
		"MARKUPP_MAX_NOTE_SIZE":   "1024",
		"MARKUPP_ALLOWED_ORIGINS": "https://painel.markupp.dev,http://localhost:5173",
	}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, urlDeTeste, cfg.DatabaseURL)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, int64(1024), cfg.MaxNoteSize)
	assert.Equal(t, []string{"https://painel.markupp.dev", "http://localhost:5173"}, cfg.AllowedOrigins)
}

func TestLoad_OrigensComEspacos_AparaCadaOrigem(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_ALLOWED_ORIGINS": " https://a.dev , https://b.dev "}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, []string{"https://a.dev", "https://b.dev"}, cfg.AllowedOrigins)
}

func TestLoad_OrigensVazia_MantemCORSDesligado(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_ALLOWED_ORIGINS": ""}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Empty(t, cfg.AllowedOrigins)
}

func TestLoad_PortaNaoNumerica_RetornaErroComOValor(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_PORT": "oitenta"}

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_PORT")
	assert.Contains(t, err.Error(), "oitenta")
}

func TestLoad_TamanhoNaoNumerico_RetornaErroComOValor(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_MAX_NOTE_SIZE": "50MB"}

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_MAX_NOTE_SIZE")
	assert.Contains(t, err.Error(), "50MB")
}
