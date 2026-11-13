package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
)

func TestLoad_SemAdministradoresDaInstancia_ListaVazia(t *testing.T) {
	cfg, err := config.Load(fakeEnv{"MARKUPP_DATABASE_URL": "postgres://x"}.Lookup)

	require.NoError(t, err)
	assert.Empty(t, cfg.InstanceAdmins)
}

func TestLoad_AdministradoresDaInstancia_AparaEPoeEmMinusculas(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": "postgres://x", "MARKUPP_INSTANCE_ADMINS": " Ana@Exemplo.org , ,bia@exemplo.org"}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, []string{"ana@exemplo.org", "bia@exemplo.org"}, cfg.InstanceAdmins)
}

func TestLoad_AdministradorDaInstanciaForaDoFormato_RetornaErroComOValor(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": "postgres://x", "MARKUPP_INSTANCE_ADMINS": "ana@exemplo.org,sem-arroba"}

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"sem-arroba"`)
	assert.Contains(t, err.Error(), "MARKUPP_INSTANCE_ADMINS")
}

func TestLoadIndexer_ChaveSecreta_LeDaVariavel(t *testing.T) {
	chave := strings.Repeat("k", 32)

	cfg, err := config.LoadIndexer(fakeEnv{"MARKUPP_DATABASE_URL": "postgres://x", "MARKUPP_SECRET_KEY": chave}.Lookup)

	require.NoError(t, err)
	assert.Equal(t, chave, cfg.SecretKey)
}

func TestLoadIndexer_ChaveSecretaCurta_RetornaErroComTamanhoSemAChave(t *testing.T) {
	_, err := config.LoadIndexer(fakeEnv{"MARKUPP_DATABASE_URL": "postgres://x", "MARKUPP_SECRET_KEY": "curta-demais"}.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_SECRET_KEY has 12 bytes")
	assert.NotContains(t, err.Error(), "curta-demais")
}
