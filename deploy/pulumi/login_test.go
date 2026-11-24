package main

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func argsComLoginExterno() MarkuppArgs {
	args := argsDeTeste()
	args.LoginProviders = []LoginProviderArgs{
		{ID: "google", Name: "Google", Issuer: "https://accounts.google.com", ClientID: "id-google", ClientSecret: pulumi.String("segredo-google")},
		{ID: "github", Name: "GitHub", ClientID: "id-github", ClientSecret: pulumi.String("segredo-github")},
	}
	return args
}

func TestNewMarkupp_SemLoginExterno_APINaoRecebeProvedores(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.NotContains(t, envNames(api), "MARKUPP_LOGIN_PROVIDERS")
}

func TestNewMarkupp_ComLoginExterno_APIRecebeOsProvedoresEOsDadosPublicos(t *testing.T) {
	monitor := deploy(t, argsComLoginExterno())

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.Equal(t, "google,github", envValue(t, api, "MARKUPP_LOGIN_PROVIDERS"))
	assert.Equal(t, "https://accounts.google.com", envValue(t, api, "MARKUPP_LOGIN_GOOGLE_ISSUER"))
	assert.Equal(t, "Google", envValue(t, api, "MARKUPP_LOGIN_GOOGLE_NAME"))
	assert.Equal(t, "id-google", envValue(t, api, "MARKUPP_LOGIN_GOOGLE_CLIENT_ID"))
	assert.Equal(t, "id-github", envValue(t, api, "MARKUPP_LOGIN_GITHUB_CLIENT_ID"))
	assert.NotContains(t, envNames(api), "MARKUPP_LOGIN_GITHUB_ISSUER", "o GitHub não fala OIDC")
}

// O segredo do cliente fica num Secret próprio: somá-lo ao Secret do banco o substituiria.
func TestNewMarkupp_ComLoginExterno_SegredoDoClienteVemDoSecretDoLogin(t *testing.T) {
	monitor := deploy(t, argsComLoginExterno())

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	ref := at(t, envNamed(t, api, "MARKUPP_LOGIN_GITHUB_CLIENT_SECRET"), "valueFrom", "secretKeyRef").ObjectValue()
	assert.Equal(t, "markupp-login", at(t, ref, "name").StringValue())
	secret := secretData(t, monitor, "markupp-login")
	assert.Equal(t, "segredo-github", secret[at(t, ref, "key").StringValue()])
}

func TestNewMarkupp_ProvedorComHifen_VariavelUsaSublinhado(t *testing.T) {
	args := argsDeTeste()
	args.LoginProviders = []LoginProviderArgs{{ID: "auth-ifsc", Issuer: "https://sso.exemplo.br", ClientID: "id", ClientSecret: pulumi.String("s")}}

	monitor := deploy(t, args)

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.Equal(t, "id", envValue(t, api, "MARKUPP_LOGIN_AUTH_IFSC_CLIENT_ID"))
}

func TestNewMarkupp_ProvedorSemSegredo_RetornaErroComOId(t *testing.T) {
	args := argsDeTeste()
	args.LoginProviders = []LoginProviderArgs{{ID: "github", ClientID: "id-github"}}

	err := declareErr(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "loginProviders")
	assert.Contains(t, err.Error(), "github")
}

func secretData(t *testing.T, monitor *fakeResourceMonitor, name string) map[string]string {
	t.Helper()
	for _, s := range monitor.byType("kubernetes:core/v1:Secret") {
		if s.Name != name {
			continue
		}
		data := map[string]string{}
		for k, v := range plain(at(t, s.Inputs, "stringData")).ObjectValue() {
			data[string(k)] = plain(v).StringValue()
		}
		return data
	}
	require.Failf(t, "Secret ausente", "esperado o Secret %q", name)
	return nil
}
