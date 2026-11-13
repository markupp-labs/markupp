package config_test

import (
	"testing"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// envComProvedores monta um ambiente valido com Google (OIDC) e GitHub; o teste troca o que precisar.
func envComProvedores() fakeEnv {
	return fakeEnv{
		"MARKUPP_DATABASE_URL":               urlDeTeste,
		"MARKUPP_LOGIN_PROVIDERS":            "google, github",
		"MARKUPP_LOGIN_GOOGLE_ISSUER":        "https://accounts.google.com",
		"MARKUPP_LOGIN_GOOGLE_CLIENT_ID":     "id-google",
		"MARKUPP_LOGIN_GOOGLE_CLIENT_SECRET": "segredo-google",
		"MARKUPP_LOGIN_GOOGLE_NAME":          "Google",
		"MARKUPP_LOGIN_GITHUB_CLIENT_ID":     "id-github",
		"MARKUPP_LOGIN_GITHUB_CLIENT_SECRET": "segredo-github",
	}
}

func TestLoad_SemProvedores_NaoTemLoginExterno(t *testing.T) {
	cfg, err := config.Load(fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste}.Lookup)

	require.NoError(t, err)
	assert.Empty(t, cfg.LoginProviders)
}

func TestLoad_ComProvedores_LeCadaUmNaOrdemDaLista(t *testing.T) {
	cfg, err := config.Load(envComProvedores().Lookup)

	require.NoError(t, err)
	assert.Equal(t, []config.LoginProvider{
		{ID: "google", Name: "Google", Issuer: "https://accounts.google.com", ClientID: "id-google", ClientSecret: "segredo-google"},
		{ID: "github", Name: "github", ClientID: "id-github", ClientSecret: "segredo-github"},
	}, cfg.LoginProviders)
}

func TestLoad_IdComHifen_LeAsVariaveisComSublinhado(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL":                urlDeTeste,
		"MARKUPP_LOGIN_PROVIDERS":             "meu-idp",
		"MARKUPP_LOGIN_MEU_IDP_ISSUER":        "https://idp.exemplo.org/application/o/markupp/",
		"MARKUPP_LOGIN_MEU_IDP_CLIENT_ID":     "id",
		"MARKUPP_LOGIN_MEU_IDP_CLIENT_SECRET": "segredo",
	}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	require.Len(t, cfg.LoginProviders, 1)
	assert.Equal(t, "https://idp.exemplo.org/application/o/markupp/", cfg.LoginProviders[0].Issuer,
		"o issuer fica como veio: o OIDC compara o texto exato com o da descoberta")
}

func TestLoad_ProvedorSemClientSecret_NomeiaAVariavel(t *testing.T) {
	env := envComProvedores()
	delete(env, "MARKUPP_LOGIN_GITHUB_CLIENT_SECRET")

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_LOGIN_GITHUB_CLIENT_SECRET")
}

func TestLoad_ProvedorSemClientID_NomeiaAVariavel(t *testing.T) {
	env := envComProvedores()
	delete(env, "MARKUPP_LOGIN_GOOGLE_CLIENT_ID")

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_LOGIN_GOOGLE_CLIENT_ID")
}

func TestLoad_ProvedorSemIssuerQueNaoEGithub_Recusa(t *testing.T) {
	env := envComProvedores()
	delete(env, "MARKUPP_LOGIN_GOOGLE_ISSUER")

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_LOGIN_GOOGLE_ISSUER")
}

func TestLoad_IssuerInseguro_Recusa(t *testing.T) {
	env := envComProvedores()
	env["MARKUPP_LOGIN_GOOGLE_ISSUER"] = "http://accounts.google.com"

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_LOGIN_GOOGLE_ISSUER")
}

func TestLoad_IdInvalido_Recusa(t *testing.T) {
	env := envComProvedores()
	env["MARKUPP_LOGIN_PROVIDERS"] = "Google"

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"Google"`)
}

func TestLoad_IdRepetido_Recusa(t *testing.T) {
	env := envComProvedores()
	env["MARKUPP_LOGIN_PROVIDERS"] = "google,github,google"

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"google"`)
}

func TestLoad_IdLocalReservado_Recusa(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL":              urlDeTeste,
		"MARKUPP_LOGIN_PROVIDERS":           "local",
		"MARKUPP_LOGIN_LOCAL_ISSUER":        "https://idp.exemplo.org",
		"MARKUPP_LOGIN_LOCAL_CLIENT_ID":     "id",
		"MARKUPP_LOGIN_LOCAL_CLIENT_SECRET": "segredo",
	}

	_, err := config.Load(env.Lookup)

	require.Error(t, err, "local e a credencial de email e senha")
	assert.Contains(t, err.Error(), `"local"`)
}

func TestLoginProvider_RedirectURL_ApontaParaOCallbackDoPainel(t *testing.T) {
	env := envComProvedores()
	env["MARKUPP_PANEL_URL"] = "https://markupp.dev/painel/"

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, "https://markupp.dev/painel/auth/google/callback", cfg.LoginRedirectURL("google"))
}
