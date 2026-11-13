package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

func TestLoginSettingsFrom_LevaCadaProvedorComOCallbackDoPainel(t *testing.T) {
	cfg := config.Config{PanelURL: "https://painel.exemplo.org", LoginProviders: []config.LoginProvider{
		{ID: "google", Name: "Google", Issuer: "https://accounts.google.com", ClientID: "a", ClientSecret: "b"},
		{ID: "github", Name: "GitHub", ClientID: "c", ClientSecret: "d"},
	}}

	got := loginSettingsFrom(cfg)

	assert.Equal(t, []loginprovider.Settings{
		{ID: "google", Name: "Google", Issuer: "https://accounts.google.com", ClientID: "a", ClientSecret: "b",
			RedirectURL: "https://painel.exemplo.org/auth/google/callback"},
		{ID: "github", Name: "GitHub", ClientID: "c", ClientSecret: "d",
			RedirectURL: "https://painel.exemplo.org/auth/github/callback"},
	}, got)
}

func TestLoginSettingsFrom_SemProvedores_ListaVazia(t *testing.T) {
	assert.Empty(t, loginSettingsFrom(config.Config{}))
}

func TestLoginProviderClient_TemTimeout(t *testing.T) {
	assert.Positive(t, loginProviderClient().Timeout, "provedor travado nao pode prender a requisicao do login")
}
