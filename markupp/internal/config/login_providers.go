package config

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// LoginProvider é um provedor externo de login (ADR-0054). Com Issuer é OpenID Connect;
// sem ele, só o id github vale, e usa o adaptador do GitHub.
type LoginProvider struct {
	ID           string
	Name         string
	Issuer       string
	ClientID     string
	ClientSecret string
}

// GitHubLoginProviderID é o único id aceito sem issuer: o GitHub não fala OIDC.
const GitHubLoginProviderID = "github"

// localIdentityProvider é o provedor da credencial de email e senha, que nenhum
// provedor externo pode reusar.
const localIdentityProvider = "local"

var loginProviderIDPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// LoginRedirectURL é o callback do painel que recebe o navegador de volta do provedor id.
func (c Config) LoginRedirectURL(id string) string {
	return strings.TrimSuffix(c.PanelURL, "/") + "/auth/" + id + "/callback"
}

// loadLoginProviders lê MARKUPP_LOGIN_PROVIDERS e as variáveis de cada provedor listado.
func loadLoginProviders(lookup LookupFunc, cfg *Config) error {
	raw, _ := lookup("MARKUPP_LOGIN_PROVIDERS")
	for _, item := range strings.Split(raw, ",") {
		id := strings.TrimSpace(item)
		if id == "" {
			continue
		}
		if err := checkLoginProviderID(id, cfg.LoginProviders); err != nil {
			return err
		}
		provider, err := readLoginProvider(lookup, id)
		if err != nil {
			return err
		}
		cfg.LoginProviders = append(cfg.LoginProviders, provider)
	}
	return nil
}

// checkLoginProviderID recusa id fora do padrão, reservado ou repetido.
func checkLoginProviderID(id string, seen []LoginProvider) error {
	if !loginProviderIDPattern.MatchString(id) || id == localIdentityProvider {
		return fmt.Errorf("MARKUPP_LOGIN_PROVIDERS has %q, expected ids of lowercase letters, digits and hyphens, other than %q", id, localIdentityProvider)
	}
	if slices.ContainsFunc(seen, func(p LoginProvider) bool { return p.ID == id }) {
		return fmt.Errorf("MARKUPP_LOGIN_PROVIDERS repeats %q, expected each id only once", id)
	}
	return nil
}

// readLoginProvider lê MARKUPP_LOGIN_<ID>_*, com o id em maiúsculas e hífen virando sublinhado.
func readLoginProvider(lookup LookupFunc, id string) (LoginProvider, error) {
	prefix := "MARKUPP_LOGIN_" + strings.ToUpper(strings.ReplaceAll(id, "-", "_")) + "_"
	provider := LoginProvider{ID: id, Name: id}
	readString(lookup, prefix+"NAME", &provider.Name)
	readString(lookup, prefix+"ISSUER", &provider.Issuer)
	readString(lookup, prefix+"CLIENT_ID", &provider.ClientID)
	readString(lookup, prefix+"CLIENT_SECRET", &provider.ClientSecret)
	for _, key := range []string{"CLIENT_ID", "CLIENT_SECRET"} {
		if value, _ := lookup(prefix + key); strings.TrimSpace(value) == "" {
			return LoginProvider{}, fmt.Errorf("%s%s is empty, expected the value registered at the provider %q", prefix, key, id)
		}
	}
	return provider, checkLoginIssuer(prefix+"ISSUER", provider)
}

// checkLoginIssuer exige issuer https (http só em loopback) a todo provedor que não é o
// GitHub. O issuer não é normalizado: o OIDC o compara letra a letra com a descoberta.
func checkLoginIssuer(key string, provider LoginProvider) error {
	if provider.Issuer == "" && provider.ID == GitHubLoginProviderID {
		return nil
	}
	parsed, err := url.Parse(provider.Issuer)
	if provider.Issuer == "" || err != nil || !oauthURLAllowed(parsed, oauthURLRule{allowPath: true}) {
		return fmt.Errorf("%s=%q, expected the OpenID Connect issuer https://host[/path] (http only on localhost); only the id %q goes without issuer",
			key, provider.Issuer, GitHubLoginProviderID)
	}
	return nil
}
