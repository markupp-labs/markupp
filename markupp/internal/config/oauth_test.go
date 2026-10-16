package config_test

import (
	"strconv"
	"strings"
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
	assert.Contains(t, err.Error(), "par 1")
	assert.Contains(t, err.Error(), "12 bytes")
	assert.Contains(t, err.Error(), "32 bytes")
	assert.NotContains(t, err.Error(), "curto-demais")
}

func TestLoad_RecursoForaDoFormato_RetornaErroComAPosicaoSemOPar(t *testing.T) {
	invalidas := []string{
		"http://mcp.markupp.dev/mcp", "mcp.markupp.dev", "https://mcp.markupp.dev/mcp#topo",
		"https://usuario@mcp.markupp.dev", "https://mcp.markupp.dev/mcp?a", "https://mcp.markupp.dev/mcp?",
		"https://:8443/mcp",
	}
	for _, valor := range invalidas {
		env := envComOAuth()
		env["MARKUPP_OAUTH_RESOURCES"] = "https://api.markupp.dev=" + outroSegredoDeTeste + "," + valor + "=" + segredoDoRecursoDeTeste

		_, err := config.Load(env.Lookup)

		require.Error(t, err, valor)
		assert.Contains(t, err.Error(), "MARKUPP_OAUTH_RESOURCES", valor)
		assert.Contains(t, err.Error(), "par 2", valor)
		assert.Contains(t, err.Error(), "url=segredo", valor)
		assert.NotContains(t, err.Error(), "mcp.markupp.dev", valor)
		assert.NotContains(t, err.Error(), segredoDoRecursoDeTeste, valor)
	}
}

func TestLoad_RecursoSoComSegredoBase64_RetornaErroSemVazarOSegredo(t *testing.T) {
	segredo := "c2VncmVkby1kby1yZWN1cnNvLWNvbS0zMi1ieXRlcyE=" // #nosec G101 -- valor ficticio de teste
	env := envComOAuth()
	env["MARKUPP_OAUTH_RESOURCES"] = segredo + "="

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "par 1")
	assert.Contains(t, err.Error(), "url=segredo")
	assert.NotContains(t, err.Error(), strings.TrimRight(segredo, "="))
}

func TestLoad_RecursoInvertido_RetornaErroSemVazarOSegredo(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_OAUTH_RESOURCES"] = segredoDoRecursoDeTeste + "=https://mcp.markupp.dev/mcp"

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "par 1")
	assert.Contains(t, err.Error(), "url=segredo")
	assert.NotContains(t, err.Error(), segredoDoRecursoDeTeste)
}

func TestLoad_RecursoRepetido_RetornaErroComAsPosicoes(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_OAUTH_RESOURCES"] = "https://mcp.markupp.dev/mcp=" + segredoDoRecursoDeTeste +
		",https://mcp.markupp.dev/mcp=" + outroSegredoDeTeste

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_OAUTH_RESOURCES")
	assert.Contains(t, err.Error(), "par 2")
	assert.Contains(t, err.Error(), "par 1")
	assert.NotContains(t, err.Error(), outroSegredoDeTeste)
}

func TestLoad_RecursoRepetidoAposNormalizar_RetornaErro(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_OAUTH_RESOURCES"] = "https://MCP.Markupp.dev/mcp/=" + segredoDoRecursoDeTeste +
		",HTTPS://mcp.markupp.dev/mcp=" + outroSegredoDeTeste

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "par 2")
}

func TestLoad_URLsComMaiusculasEBarraFinal_Normaliza(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_PUBLIC_URL"] = "HTTPS://Notas.Markupp.DEV/"
	env["MARKUPP_PANEL_URL"] = "https://Markupp.dev/Painel/"
	env["MARKUPP_OAUTH_RESOURCES"] = "https://MCP.markupp.dev/Mcp/=" + segredoDoRecursoDeTeste

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, "https://notas.markupp.dev", cfg.PublicURL)
	assert.Equal(t, "https://markupp.dev/Painel", cfg.PanelURL, "o caminho preserva a caixa")
	assert.Equal(t, "https://mcp.markupp.dev/Mcp", cfg.OAuthResources[0].URL)
}

func TestLoad_HTTPEmLoopbackMaiusculo_AceitaENormaliza(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_PUBLIC_URL"] = "HTTP://LOCALHOST:8080"
	env["MARKUPP_PANEL_URL"] = "http://LocalHost:3999/painel"
	env["MARKUPP_OAUTH_RESOURCES"] = "HTTP://LOCALHOST:8090/mcp=" + segredoDoRecursoDeTeste

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", cfg.PublicURL)
	assert.Equal(t, "http://localhost:3999/painel", cfg.PanelURL)
	assert.Equal(t, "http://localhost:8090/mcp", cfg.OAuthResources[0].URL)
}

func TestLoad_URLComHostVazio_RetornaErro(t *testing.T) {
	for _, chave := range []string{"MARKUPP_PUBLIC_URL", "MARKUPP_PANEL_URL"} {
		env := envComOAuth()
		env[chave] = "https://:8443"

		_, err := config.Load(env.Lookup)

		require.Error(t, err, chave)
		assert.Contains(t, err.Error(), chave)
	}
}

func TestLoad_URLsComPortaPadrao_RemoveAPorta(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_PUBLIC_URL"] = "https://notas.markupp.dev:443"
	env["MARKUPP_PANEL_URL"] = "http://localhost:80/painel"
	env["MARKUPP_OAUTH_RESOURCES"] = "https://mcp.markupp.dev:443/mcp=" + segredoDoRecursoDeTeste +
		",http://[::1]:80/mcp=" + outroSegredoDeTeste

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, "https://notas.markupp.dev", cfg.PublicURL)
	assert.Equal(t, "http://localhost/painel", cfg.PanelURL)
	assert.Equal(t, "https://mcp.markupp.dev/mcp", cfg.OAuthResources[0].URL)
	assert.Equal(t, "http://[::1]/mcp", cfg.OAuthResources[1].URL)
}

func TestLoad_URLsComPortaPadraoDoOutroEsquema_MantemAPorta(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_PUBLIC_URL"] = "https://notas.markupp.dev:80"
	env["MARKUPP_PANEL_URL"] = "http://localhost:443/painel"

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, "https://notas.markupp.dev:80", cfg.PublicURL)
	assert.Equal(t, "http://localhost:443/painel", cfg.PanelURL)
}

func TestLoad_RecursoRepetidoComPortaPadrao_RetornaErro(t *testing.T) {
	env := envComOAuth()
	env["MARKUPP_OAUTH_RESOURCES"] = "https://mcp.markupp.dev/mcp=" + segredoDoRecursoDeTeste +
		",https://mcp.markupp.dev:443/mcp=" + outroSegredoDeTeste

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "par 2")
}

func TestDefault_DuracoesDoOAuth(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, 60, cfg.OAuthAccessMinutes)
	assert.Equal(t, 30, cfg.OAuthRefreshDays)
	assert.Equal(t, 10, cfg.OAuthRequestMinutes)
	assert.Equal(t, 60, cfg.OAuthCodeSeconds)
}

func TestLoad_OAuthSemDuracoes_UsaOPadrao(t *testing.T) {
	cfg, err := config.Load(envComOAuth().Lookup)

	require.NoError(t, err)
	assert.Equal(t, 60, cfg.OAuthAccessMinutes)
	assert.Equal(t, 30, cfg.OAuthRefreshDays)
	assert.Equal(t, 10, cfg.OAuthRequestMinutes)
	assert.Equal(t, 60, cfg.OAuthCodeSeconds)
}

func TestLoad_DuracoesDoOAuthNosLimites_Aceita(t *testing.T) {
	for _, limite := range []struct{ acesso, refresh, pedido, codigo int }{
		{1, 1, 1, 10}, {1440, 365, 60, 600},
	} {
		env := envComOAuth()
		env["MARKUPP_OAUTH_ACCESS_MINUTES"] = strconv.Itoa(limite.acesso)
		env["MARKUPP_OAUTH_REFRESH_DAYS"] = strconv.Itoa(limite.refresh)
		env["MARKUPP_OAUTH_REQUEST_MINUTES"] = strconv.Itoa(limite.pedido)
		env["MARKUPP_OAUTH_CODE_SECONDS"] = strconv.Itoa(limite.codigo)

		cfg, err := config.Load(env.Lookup)

		require.NoError(t, err, limite)
		assert.Equal(t, limite.acesso, cfg.OAuthAccessMinutes)
		assert.Equal(t, limite.refresh, cfg.OAuthRefreshDays)
		assert.Equal(t, limite.pedido, cfg.OAuthRequestMinutes)
		assert.Equal(t, limite.codigo, cfg.OAuthCodeSeconds)
	}
}

func TestLoad_DuracaoDoOAuthInvalida_RetornaErroComVariavelEValor(t *testing.T) {
	casos := []struct{ chave, valor, faixa string }{
		{"MARKUPP_OAUTH_ACCESS_MINUTES", "0", "de 1 a 1440"},
		{"MARKUPP_OAUTH_ACCESS_MINUTES", "1441", "de 1 a 1440"},
		{"MARKUPP_OAUTH_REFRESH_DAYS", "366", "de 1 a 365"},
		{"MARKUPP_OAUTH_REQUEST_MINUTES", "61", "de 1 a 60"},
		{"MARKUPP_OAUTH_CODE_SECONDS", "9", "de 10 a 600"},
		{"MARKUPP_OAUTH_CODE_SECONDS", "601", "de 10 a 600"},
		{"MARKUPP_OAUTH_CODE_SECONDS", "um minuto", "de 10 a 600"},
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

func TestLoad_SemPublicURL_IgnoraDuracoesDoOAuth(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido,
		"MARKUPP_OAUTH_ACCESS_MINUTES": "0", "MARKUPP_OAUTH_CODE_SECONDS": "lixo",
	}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err, "sem MARKUPP_PUBLIC_URL as duracoes do OAuth nem sao lidas")
	assert.Equal(t, 60, cfg.OAuthAccessMinutes)
	assert.Equal(t, 60, cfg.OAuthCodeSeconds)
}
