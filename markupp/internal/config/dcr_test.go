package config_test

import (
	"strconv"
	"testing"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_OAuthSemVariaveisDoDCR_UsaOPadrao(t *testing.T) {
	cfg, err := config.Load(envComOAuth().Lookup)

	require.NoError(t, err)
	assert.False(t, cfg.OAuthDCREnabled, "o DCR vem desligado; Claude, Cursor e VS Code usam CIMD")
	assert.Equal(t, 1000, cfg.OAuthDCRMaxPending)
	assert.Equal(t, 24, cfg.OAuthDCRPendingHours)
}

func TestLoad_DCRLigado_LeOBooleano(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_OAUTH_DCR_ENABLED"] = "true"

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.True(t, cfg.OAuthDCREnabled)
}

func TestLoad_LimitesDoDCRNasBordas_Aceita(t *testing.T) {
	for _, limite := range []struct{ maxPending, pendingHours int }{{1, 1}, {100000, 168}} {
		env := envComOAuth()
		env["MARKUPP_OAUTH_DCR_MAX_PENDING"] = strconv.Itoa(limite.maxPending)
		env["MARKUPP_OAUTH_DCR_PENDING_HOURS"] = strconv.Itoa(limite.pendingHours)

		cfg, err := config.Load(env.Lookup)

		require.NoError(t, err, limite)
		assert.Equal(t, limite.maxPending, cfg.OAuthDCRMaxPending)
		assert.Equal(t, limite.pendingHours, cfg.OAuthDCRPendingHours)
	}
}

func TestLoad_VariavelDoDCRInvalida_RetornaErroComVariavelEValor(t *testing.T) {
	casos := []struct{ chave, valor, esperado string }{
		{"MARKUPP_OAUTH_DCR_MAX_PENDING", "0", "de 1 a 100000"},
		{"MARKUPP_OAUTH_DCR_MAX_PENDING", "100001", "de 1 a 100000"},
		{"MARKUPP_OAUTH_DCR_PENDING_HOURS", "0", "de 1 a 168"},
		{"MARKUPP_OAUTH_DCR_PENDING_HOURS", "169", "de 1 a 168"},
		{"MARKUPP_OAUTH_DCR_PENDING_HOURS", "um dia", "de 1 a 168"},
		{"MARKUPP_OAUTH_DCR_ENABLED", "talvez", "booleano"},
	}
	for _, caso := range casos {
		env := envComOAuth()
		env[caso.chave] = caso.valor

		_, err := config.Load(env.Lookup)

		require.Error(t, err, caso)
		assert.Contains(t, err.Error(), caso.chave)
		assert.Contains(t, err.Error(), caso.valor)
		assert.Contains(t, err.Error(), caso.esperado)
	}
}

func TestLoad_SemPublicURL_AindaLeAsVariaveisDoDCR(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_OAUTH_DCR_ENABLED": "talvez"}

	_, err := config.Load(env.Lookup)

	require.Error(t, err, "o OAuth vem ligado pela URL publica padrao")
	assert.Contains(t, err.Error(), "MARKUPP_OAUTH_DCR_ENABLED")
}
