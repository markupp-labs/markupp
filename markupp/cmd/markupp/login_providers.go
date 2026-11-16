package main

import (
	"net/http"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

// loginProviderTimeout limita cada chamada ao provedor externo de login.
const loginProviderTimeout = 10 * time.Second

// newExternalLoginService monta o login externo (ADR-0054). Sem provedor configurado,
// a lista sai vazia e o painel não mostra botão.
func newExternalLoginService(cfg config.Config, base *auth.Service) *auth.ExternalLoginService {
	registry := loginprovider.NewRegistry(loginSettingsFrom(cfg), loginProviderClient())
	return auth.NewExternalLoginService(base, registry)
}

// loginSettingsFrom traduz os provedores da config, com o callback do painel de cada um.
func loginSettingsFrom(cfg config.Config) []loginprovider.Settings {
	settings := make([]loginprovider.Settings, 0, len(cfg.LoginProviders))
	for _, p := range cfg.LoginProviders {
		settings = append(settings, loginprovider.Settings{
			ID: p.ID, Name: p.Name, Issuer: p.Issuer, ClientID: p.ClientID, ClientSecret: p.ClientSecret,
			RedirectURL: cfg.LoginRedirectURL(p.ID),
		})
	}
	return settings
}

// loginProviderClient fala com os provedores com timeout, para um provedor travado não
// prender a requisição de login.
func loginProviderClient() *http.Client {
	return &http.Client{Timeout: loginProviderTimeout}
}
