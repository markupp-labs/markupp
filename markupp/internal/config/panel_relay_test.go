package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
)

const segredoDoPainelDeTeste = "segredo-de-relay-do-painel-com-32-bytes" // #nosec G101 -- valor fictício de teste

func TestLoad_LeOSegredoDeRelayDoPainelEOArquivoDele(t *testing.T) {
	cfg := carregar(t, fakeEnv{ // #nosec G101 -- valores fictícios de teste
		"MARKUPP_DATABASE_URL":            urlDeTeste,
		"MARKUPP_PANEL_RELAY_SECRET":      segredoDoPainelDeTeste,
		"MARKUPP_PANEL_RELAY_SECRET_PATH": "/run/markupp-panel/secret",
	})

	assert.Equal(t, segredoDoPainelDeTeste, cfg.PanelRelaySecret)
	assert.Equal(t, "/run/markupp-panel/secret", cfg.PanelRelaySecretPath)
}

func TestLoad_SegredoDeRelayDoPainelCurto_RetornaErroComOTamanho(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_PANEL_RELAY_SECRET": "curto"}

	_, err := config.Load(env.Lookup)

	require.ErrorContains(t, err, "MARKUPP_PANEL_RELAY_SECRET has 5 bytes")
}

func TestWithInstanceSecrets_ComOAuthDosResources_AindaGeraOSegredoDoPainel(t *testing.T) {
	secrets := newFakeInstanceSecrets()

	resolved, err := carregar(t, envComOAuth()).WithInstanceSecrets(context.Background(), secrets, gerarSegredo)

	require.NoError(t, err)
	assert.Equal(t, "segredo-gerado-com-mais-de-32-bytes!", resolved.PanelRelaySecret)
	assert.Equal(t, []string{"panel_relay_secret"}, secrets.generated)
}

func TestWithInstanceSecrets_ComSegredoDoPainelNaVariavel_NaoGera(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_PANEL_RELAY_SECRET"] = segredoDoPainelDeTeste
	secrets := newFakeInstanceSecrets()

	resolved, err := carregar(t, env).WithInstanceSecrets(context.Background(), secrets, gerarSegredo)

	require.NoError(t, err)
	assert.Equal(t, segredoDoPainelDeTeste, resolved.PanelRelaySecret)
	assert.Empty(t, secrets.generated)
}
