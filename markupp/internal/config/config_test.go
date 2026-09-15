package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
)

func TestDefault_RetornaValoresPadrao(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "./markupp.db", cfg.DBPath)
	assert.Equal(t, int64(50*1024*1024), cfg.MaxNoteSize)
}

func TestLoad_SemEnvVarSemArquivo_RetornaDefaults(t *testing.T) {
	t.Setenv("MARKUPP_CONFIG_PATH", "")
	t.Chdir(t.TempDir())

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.Default(), cfg)
}

func TestLoad_EnvVarApontaPathInexistente_RetornaDefaults(t *testing.T) {
	t.Setenv("MARKUPP_CONFIG_PATH", filepath.Join(t.TempDir(), "nao-existe.json"))

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.Default(), cfg)
}

func TestLoad_FallbackConfigJsonNoCwd_QuandoEnvVarVazia(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"port": 9090, "db_path": "/data/markupp.db", "max_note_size": 1024}`), 0o600))

	t.Setenv("MARKUPP_CONFIG_PATH", "")
	t.Chdir(dir)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "/data/markupp.db", cfg.DBPath)
	assert.Equal(t, int64(1024), cfg.MaxNoteSize)
}

func TestLoad_ArquivoCompleto_LeTodosOsCampos(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"port": 9090, "db_path": "/data/markupp.db", "max_note_size": 1024}`), 0o600))

	t.Setenv("MARKUPP_CONFIG_PATH", path)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "/data/markupp.db", cfg.DBPath)
	assert.Equal(t, int64(1024), cfg.MaxNoteSize)
}

func TestLoad_ArquivoParcial_PreservaDefaultsDasChavesAusentes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"port": 9090}`), 0o600))

	t.Setenv("MARKUPP_CONFIG_PATH", path)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "./markupp.db", cfg.DBPath)
	assert.Equal(t, int64(50*1024*1024), cfg.MaxNoteSize)
}

func TestLoad_JsonInvalido_RetornaErro(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{port: invalido`), 0o600))

	t.Setenv("MARKUPP_CONFIG_PATH", path)

	_, err := config.Load()

	require.Error(t, err)
}

func TestDefault_NaoTemOrigemPermitida(t *testing.T) {
	cfg := config.Default()

	assert.Empty(t, cfg.AllowedOrigins, "sem origem configurada o CORS fica desligado")
}

func TestLoad_ArquivoComOrigens_LeAsOrigensPermitidas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"allowed_origins": ["https://painel.markupp.dev", "http://localhost:5173"]}`), 0o600))
	t.Setenv("MARKUPP_CONFIG_PATH", path)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, []string{"https://painel.markupp.dev", "http://localhost:5173"}, cfg.AllowedOrigins)
}

func TestLoad_ArquivoSemOrigens_PreservaListaVazia(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"port": 9090}`), 0o600))
	t.Setenv("MARKUPP_CONFIG_PATH", path)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Empty(t, cfg.AllowedOrigins)
}
