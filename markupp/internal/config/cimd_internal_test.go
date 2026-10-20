package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// As faixas das variáveis hoje impedem o mínimo de passar do máximo pelo ambiente; o
// teste chama a checagem direto para ela continuar valendo se as faixas mudarem.
func TestCheckCIMDCacheBounds_MinimoAcimaDoMaximo_RetornaErroComOsDoisValores(t *testing.T) {
	cfg := Config{OAuthCIMDCacheMinMinutes: 61, OAuthCIMDCacheMaxHours: 1}

	err := cfg.checkCIMDCacheBounds()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_OAUTH_CIMD_CACHE_MIN_MINUTES=61")
	assert.Contains(t, err.Error(), "MARKUPP_OAUTH_CIMD_CACHE_MAX_HOURS=1")
}

func TestCheckCIMDCacheBounds_MinimoIgualAoMaximo_Aceita(t *testing.T) {
	cfg := Config{OAuthCIMDCacheMinMinutes: 60, OAuthCIMDCacheMaxHours: 1}

	assert.NoError(t, cfg.checkCIMDCacheBounds())
}
