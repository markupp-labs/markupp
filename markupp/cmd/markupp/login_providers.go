package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

const (
	// loginProviderTimeout limita cada chamada ao provedor externo de login.
	loginProviderTimeout = 10 * time.Second
	// loginProvidersRefresh é o máximo que uma réplica da API leva para ver a mudança
	// que o administrador fez na lista (ADR-0060).
	loginProvidersRefresh = 5 * time.Second
)

// newLoginProviderService monta os provedores de login do administrador, com os das
// variáveis de quem hospeda travando a lista quando definidos.
func newLoginProviderService(cfg config.Config, pool *pgxpool.Pool, sealer instance.Sealer) *instance.LoginProviderService {
	return instance.NewLoginProviderService(storage.NewInstanceLoginProviders(pool), sealer, loginSettingsFrom(cfg), cfg.LoginRedirectURL)
}

// newLoginProviderRegistry dá ao login os provedores vigentes, relidos quando mudam.
func newLoginProviderRegistry(source loginprovider.Source, logger *slog.Logger) *loginprovider.RefreshingRegistry {
	return loginprovider.NewRefreshingRegistry(source, loginProviderClient(), loginProvidersRefresh, time.Now, logger)
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
