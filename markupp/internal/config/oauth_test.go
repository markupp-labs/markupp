package config_test

import (
	"testing"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	segredoDoRecursoDeTeste = "segredo-do-recurso-com-32-bytes!" // #nosec G101 -- valor ficticio de teste
	outroSegredoDeTeste     = "outro-segredo-do-recurso-32bytes" // #nosec G101 -- valor ficticio de teste
)

// envComOAuth monta um ambiente valido com OAuth ligado; o teste troca o que precisar.
func envComOAuth() fakeEnv {
	return fakeEnv{
		"MARKUPP_DATABASE_URL":    urlDeTeste,
		"MARKUPP_AUTH_SECRET":     segredoDeTesteValido,
		"MARKUPP_PUBLIC_URL":      "https://notas.markupp.dev",
		"MARKUPP_PANEL_URL":       "https://painel.markupp.dev",
		"MARKUPP_OAUTH_RESOURCES": "https://mcp.markupp.dev/mcp=" + segredoDoRecursoDeTeste,
	}
}

func TestDefault_SemPublicURL_OAuthDesligado(t *testing.T) {
	cfg := config.Default()

	assert.False(t, cfg.OAuthEnabled())
	assert.Empty(t, cfg.PublicURL)
	assert.Empty(t, cfg.PanelURL)
	assert.Empty(t, cfg.OAuthResources)
}

func TestLoad_SemPublicURL_IgnoraPainelERecursos(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido,
		"MARKUPP_PANEL_URL": "ftp://invalido", "MARKUPP_OAUTH_RESOURCES": "lixo",
	}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err, "sem MARKUPP_PUBLIC_URL o OAuth fica desligado e o resto nem e lido")
	assert.False(t, cfg.OAuthEnabled())
	assert.Empty(t, cfg.PanelURL)
	assert.Empty(t, cfg.OAuthResources)
}

func TestLoad_OAuthCompleto_LeOsTresCampos(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_PUBLIC_URL"] = "https://notas.markupp.dev/"
	env["MARKUPP_PANEL_URL"] = "https://markupp.dev/painel/"
	env["MARKUPP_OAUTH_RESOURCES"] = " https://mcp.markupp.dev/mcp=" + segredoDoRecursoDeTeste +
		" , https://api.markupp.dev=" + outroSegredoDeTeste

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.True(t, cfg.OAuthEnabled())
	assert.Equal(t, "https://notas.markupp.dev", cfg.PublicURL, "o issuer fica sem barra final")
	assert.Equal(t, "https://markupp.dev/painel", cfg.PanelURL)
	assert.Equal(t, []config.OAuthResource{
		{URL: "https://mcp.markupp.dev/mcp", Secret: segredoDoRecursoDeTeste},
		{URL: "https://api.markupp.dev", Secret: outroSegredoDeTeste},
	}, cfg.OAuthResources)
}

func TestLoad_HTTPEmLoopback_Aceita(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "[::1]"} {
		env := envComOAuth()
		env["MARKUPP_PUBLIC_URL"] = "http://" + host + ":8080"
		env["MARKUPP_PANEL_URL"] = "http://" + host + ":3999/painel"
		env["MARKUPP_OAUTH_RESOURCES"] = "http://" + host + ":8090/mcp=" + segredoDoRecursoDeTeste

		cfg, err := config.Load(env.Lookup)

		require.NoError(t, err, host)
		assert.Equal(t, "http://"+host+":8080", cfg.PublicURL)
	}
}

func TestLoad_PublicURLForaDoFormato_RetornaErroComOValor(t *testing.T) {
	invalidas := []string{
		"http://notas.markupp.dev", "ftp://notas.markupp.dev", "notas.markupp.dev", "https://",
		"https://notas.markupp.dev/v1", "https://notas.markupp.dev?a=1", "https://notas.markupp.dev#topo",
		"https://usuario@notas.markupp.dev", "http://localhost.markupp.dev",
	}
	for _, valor := range invalidas {
		env := envComOAuth()
		env["MARKUPP_PUBLIC_URL"] = valor

		_, err := config.Load(env.Lookup)

		require.Error(t, err, valor)
		assert.Contains(t, err.Error(), "MARKUPP_PUBLIC_URL", valor)
		assert.Contains(t, err.Error(), "https://", valor)
	}
}

func TestLoad_PublicURLComSenha_RetornaErroSemVazarASenha(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_PUBLIC_URL"] = "https://usuario:segredo@notas.markupp.dev"

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_PUBLIC_URL")
	assert.NotContains(t, err.Error(), "segredo")
}

func TestLoad_OAuthSemPainel_RetornaErroNomeandoAVariavel(t *testing.T) {
	env := envComOAuth()
	delete(env, "MARKUPP_PANEL_URL")

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_PANEL_URL")
	assert.Contains(t, err.Error(), "MARKUPP_PUBLIC_URL", "a mensagem diz por que o painel e obrigatorio")
}

func TestLoad_PainelForaDoFormato_RetornaErroComOValor(t *testing.T) {
	invalidas := []string{
		"http://painel.markupp.dev", "painel.markupp.dev", "https://painel.markupp.dev?a=1",
		"https://painel.markupp.dev/#topo",
	}
	for _, valor := range invalidas {
		env := envComOAuth()
		env["MARKUPP_PANEL_URL"] = valor

		_, err := config.Load(env.Lookup)

		require.Error(t, err, valor)
		assert.Contains(t, err.Error(), "MARKUPP_PANEL_URL", valor)
		assert.Contains(t, err.Error(), "painel.markupp.dev", valor)
	}
}

func TestLoad_OAuthSemRecursos_RetornaErroNomeandoAVariavel(t *testing.T) {
	for _, valor := range []string{"", " , "} {
		env := envComOAuth()
		env["MARKUPP_OAUTH_RESOURCES"] = valor

		_, err := config.Load(env.Lookup)

		require.Error(t, err, valor)
		assert.Contains(t, err.Error(), "MARKUPP_OAUTH_RESOURCES")
		assert.Contains(t, err.Error(), "url=segredo")
	}
}

func TestLoad_RecursoSemSegredo_RetornaErroSemVazarOPar(t *testing.T) {
	for _, valor := range []string{segredoDoRecursoDeTeste, "https://mcp.markupp.dev/mcp="} {
		env := envComOAuth()
		env["MARKUPP_OAUTH_RESOURCES"] = valor

		_, err := config.Load(env.Lookup)

		require.Error(t, err, valor)
		assert.Contains(t, err.Error(), "MARKUPP_OAUTH_RESOURCES")
		assert.Contains(t, err.Error(), "url=segredo")
		assert.NotContains(t, err.Error(), segredoDoRecursoDeTeste)
	}
}

func TestLoad_RecursoComSegredoCurto_RetornaErroComTamanhoSemOSegredo(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_OAUTH_RESOURCES"] = "https://mcp.markupp.dev/mcp=curto-demais" // #nosec G101 -- valor ficticio de teste

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_OAUTH_RESOURCES")
	assert.Contains(t, err.Error(), "https://mcp.markupp.dev/mcp")
	assert.Contains(t, err.Error(), "12 bytes")
	assert.Contains(t, err.Error(), "32 bytes")
	assert.NotContains(t, err.Error(), "curto-demais")
}

func TestLoad_RecursoForaDoFormato_RetornaErroComAURLSemOSegredo(t *testing.T) {
	invalidas := []string{
		"http://mcp.markupp.dev/mcp", "mcp.markupp.dev", "https://mcp.markupp.dev/mcp#topo",
		"https://usuario@mcp.markupp.dev",
	}
	for _, valor := range invalidas {
		env := envComOAuth()
		env["MARKUPP_OAUTH_RESOURCES"] = valor + "=" + segredoDoRecursoDeTeste

		_, err := config.Load(env.Lookup)

		require.Error(t, err, valor)
		assert.Contains(t, err.Error(), "MARKUPP_OAUTH_RESOURCES", valor)
		assert.Contains(t, err.Error(), "mcp.markupp.dev", valor)
		assert.NotContains(t, err.Error(), segredoDoRecursoDeTeste, valor)
	}
}

func TestLoad_RecursoRepetido_RetornaErroComAURL(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_OAUTH_RESOURCES"] = "https://mcp.markupp.dev/mcp=" + segredoDoRecursoDeTeste +
		",https://mcp.markupp.dev/mcp=" + outroSegredoDeTeste

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_OAUTH_RESOURCES")
	assert.Contains(t, err.Error(), "https://mcp.markupp.dev/mcp")
	assert.NotContains(t, err.Error(), outroSegredoDeTeste)
}
