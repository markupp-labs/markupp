package loginprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"
)

// GitHubEndpoints são os endereços do GitHub; o teste aponta para um servidor falso.
type GitHubEndpoints struct {
	AuthURL  string
	TokenURL string
	APIURL   string
}

// DefaultGitHubEndpoints são os do github.com.
var DefaultGitHubEndpoints = GitHubEndpoints{ // #nosec G101 -- URLs publicas, nao credenciais
	AuthURL:  "https://github.com/login/oauth/authorize",
	TokenURL: "https://github.com/login/oauth/access_token",
	APIURL:   "https://api.github.com",
}

// githubProvider é o GitHub, que fala OAuth 2.0 sem OIDC: a identidade vem da API.
type githubProvider struct {
	settings  Settings
	endpoints GitHubEndpoints
	client    *http.Client
}

// NewGitHub devolve o provedor GitHub de settings nos endpoints dados.
func NewGitHub(settings Settings, endpoints GitHubEndpoints, client *http.Client) Provider {
	return &githubProvider{settings: settings, endpoints: endpoints, client: client}
}

func (p *githubProvider) ID() string   { return p.settings.ID }
func (p *githubProvider) Name() string { return p.settings.Name }

func (p *githubProvider) oauthConfig() oauth2.Config {
	return oauth2.Config{
		ClientID: p.settings.ClientID, ClientSecret: p.settings.ClientSecret, RedirectURL: p.settings.RedirectURL,
		Endpoint: oauth2.Endpoint{AuthURL: p.endpoints.AuthURL, TokenURL: p.endpoints.TokenURL},
		Scopes:   []string{"read:user", "user:email"},
	}
}

// Begin não usa o nonce, que é do OIDC; ele volta mesmo assim e o Finish o ignora.
func (p *githubProvider) Begin(_ context.Context) (Attempt, error) {
	attempt := newAttemptValues()
	config := p.oauthConfig()
	attempt.AuthorizationURL = config.AuthCodeURL(attempt.State, oauth2.S256ChallengeOption(attempt.CodeVerifier))
	return attempt, nil
}

func (p *githubProvider) Finish(ctx context.Context, code, codeVerifier, _ string) (Identity, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	config := p.oauthConfig()
	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return Identity{}, classifyExchangeError(err)
	}
	api := config.Client(ctx, token)
	var user struct {
		ID int64 `json:"id"`
	}
	if err := p.getJSON(api, "/user", &user); err != nil {
		return Identity{}, err
	}
	email, verified, err := p.primaryEmail(api)
	if err != nil {
		return Identity{}, err
	}
	return Identity{Subject: strconv.FormatInt(user.ID, 10), Email: email, EmailVerified: verified}, nil
}

// primaryEmail devolve o email principal da conta e se o GitHub o verificou.
func (p *githubProvider) primaryEmail(api *http.Client) (string, bool, error) {
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := p.getJSON(api, "/user/emails", &emails); err != nil {
		return "", false, err
	}
	for _, e := range emails {
		if e.Primary {
			return e.Email, e.Verified, nil
		}
	}
	return "", false, nil
}

func (p *githubProvider) getJSON(api *http.Client, path string, target any) error {
	resp, err := api.Get(p.endpoints.APIURL + path)
	if err != nil {
		return fmt.Errorf("%w: GET %s: %w", ErrProviderUnavailable, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: GET %s answered %d, expected 200", ErrProviderUnavailable, path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("%w: GET %s body: %w", ErrProviderUnavailable, path, err)
	}
	return nil
}
