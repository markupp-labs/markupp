package config_test

import (
	"strconv"
	"testing"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_OAuthSemLimitesDoCIMD_UsaOPadrao(t *testing.T) {
	cfg, err := config.Load(envComOAuth().Lookup)

	require.NoError(t, err)
	assert.Equal(t, 5, cfg.OAuthCIMDTimeoutSeconds)
	assert.Equal(t, 64, cfg.OAuthCIMDMaxKB)
	assert.Equal(t, 5, cfg.OAuthCIMDCacheMinMinutes)
	assert.Equal(t, 24, cfg.OAuthCIMDCacheMaxHours)
}

func TestLoad_LimitesDoCIMDNasBordas_Aceita(t *testing.T) {
	for _, limite := range []struct{ timeout, maxKB, cacheMin, cacheMax int }{
		{1, 1, 1, 1}, {30, 1024, 60, 168}, {30, 1024, 60, 1},
	} {
		env := envComOAuth()
		env["MARKUPP_OAUTH_CIMD_TIMEOUT_SECONDS"] = strconv.Itoa(limite.timeout)
		env["MARKUPP_OAUTH_CIMD_MAX_KB"] = strconv.Itoa(limite.maxKB)
		env["MARKUPP_OAUTH_CIMD_CACHE_MIN_MINUTES"] = strconv.Itoa(limite.cacheMin)
		env["MARKUPP_OAUTH_CIMD_CACHE_MAX_HOURS"] = strconv.Itoa(limite.cacheMax)

		cfg, err := config.Load(env.Lookup)

		require.NoError(t, err, limite)
		assert.Equal(t, limite.timeout, cfg.OAuthCIMDTimeoutSeconds)
		assert.Equal(t, limite.maxKB, cfg.OAuthCIMDMaxKB)
		assert.Equal(t, limite.cacheMin, cfg.OAuthCIMDCacheMinMinutes)
		assert.Equal(t, limite.cacheMax, cfg.OAuthCIMDCacheMaxHours)
	}
}

func TestLoad_LimiteDoCIMDInvalido_RetornaErroComVariavelEValor(t *testing.T) {
	casos := []struct{ chave, valor, faixa string }{
		{"MARKUPP_OAUTH_CIMD_TIMEOUT_SECONDS", "0", "de 1 a 30"},
		{"MARKUPP_OAUTH_CIMD_TIMEOUT_SECONDS", "31", "de 1 a 30"},
		{"MARKUPP_OAUTH_CIMD_MAX_KB", "0", "de 1 a 1024"},
		{"MARKUPP_OAUTH_CIMD_MAX_KB", "1025", "de 1 a 1024"},
		{"MARKUPP_OAUTH_CIMD_CACHE_MIN_MINUTES", "0", "de 1 a 60"},
		{"MARKUPP_OAUTH_CIMD_CACHE_MIN_MINUTES", "61", "de 1 a 60"},
		{"MARKUPP_OAUTH_CIMD_CACHE_MAX_HOURS", "0", "de 1 a 168"},
		{"MARKUPP_OAUTH_CIMD_CACHE_MAX_HOURS", "169", "de 1 a 168"},
		{"MARKUPP_OAUTH_CIMD_CACHE_MAX_HOURS", "um dia", "de 1 a 168"},
	}
	for _, caso := range casos {
		env := envComOAuth()
		env[caso.chave] = caso.valor

		_, err := config.Load(env.Lookup)

		require.Error(t, err, caso)
		assert.Contains(t, err.Error(), caso.chave)
		assert.Contains(t, err.Error(), caso.valor)
		assert.Contains(t, err.Error(), caso.faixa)
	}
}

func TestLoad_SemPublicURL_AindaLeOsLimitesDoCIMD(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_OAUTH_CIMD_TIMEOUT_SECONDS": "0"}

	_, err := config.Load(env.Lookup)

	require.Error(t, err, "o OAuth vem ligado pela URL publica padrao")
	assert.Contains(t, err.Error(), "MARKUPP_OAUTH_CIMD_TIMEOUT_SECONDS")
}
