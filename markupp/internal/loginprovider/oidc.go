package loginprovider

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// oidcProvider é um provedor OpenID Connect. A descoberta é preguiçosa e fica em cache
// depois do primeiro sucesso, para o IdP fora do ar não impedir a subida.
type oidcProvider struct {
	settings Settings
	client   *http.Client
	mu       sync.Mutex
	found    *oidc.Provider
}

// NewOIDC devolve o provedor OIDC de settings, que fala com o IdP por client.
func NewOIDC(settings Settings, client *http.Client) Provider {
	return &oidcProvider{settings: settings, client: client}
}

func (p *oidcProvider) ID() string   { return p.settings.ID }
func (p *oidcProvider) Name() string { return p.settings.Name }

func (p *oidcProvider) discover(ctx context.Context) (*oidc.Provider, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.found != nil {
		return p.found, nil
	}
	found, err := oidc.NewProvider(oidc.ClientContext(ctx, p.client), p.settings.Issuer)
	if err != nil {
		return nil, fmt.Errorf("%w: discover issuer %q: %w", ErrProviderUnavailable, p.settings.Issuer, err)
	}
	p.found = found
	return found, nil
}

func (p *oidcProvider) oauthConfig(found *oidc.Provider) oauth2.Config {
	return oauth2.Config{
		ClientID: p.settings.ClientID, ClientSecret: p.settings.ClientSecret,
		Endpoint: found.Endpoint(), RedirectURL: p.settings.RedirectURL,
		Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
	}
}

func (p *oidcProvider) Begin(ctx context.Context) (Attempt, error) {
	found, err := p.discover(ctx)
	if err != nil {
		return Attempt{}, err
	}
	attempt := newAttemptValues()
	config := p.oauthConfig(found)
	attempt.AuthorizationURL = config.AuthCodeURL(attempt.State,
		oauth2.S256ChallengeOption(attempt.CodeVerifier), oidc.Nonce(attempt.Nonce))
	return attempt, nil
}

func (p *oidcProvider) Finish(ctx context.Context, code, codeVerifier, nonce string) (Identity, error) {
	if nonce == "" {
		return Identity{}, fmt.Errorf("%w: empty nonce, expected the one returned by Begin", ErrLoginRejected)
	}
	found, err := p.discover(ctx)
	if err != nil {
		return Identity{}, err
	}
	ctx = oidc.ClientContext(ctx, p.client)
	config := p.oauthConfig(found)
	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return Identity{}, classifyExchangeError(err)
	}
	return p.verifyIDToken(ctx, found, token, nonce)
}

// verifyIDToken confere assinatura, emissor, audiência, validade e nonce do id_token.
func (p *oidcProvider) verifyIDToken(ctx context.Context, found *oidc.Provider, token *oauth2.Token, nonce string) (Identity, error) {
	raw, _ := token.Extra("id_token").(string)
	idToken, err := found.Verifier(&oidc.Config{ClientID: p.settings.ClientID}).Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: id_token from %q: %w", ErrLoginRejected, p.settings.ID, err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return Identity{}, fmt.Errorf("%w: id_token nonce differs from the attempt of %q", ErrLoginRejected, p.settings.ID)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("%w: id_token claims from %q: %w", ErrLoginRejected, p.settings.ID, err)
	}
	return Identity{Subject: idToken.Subject, Email: claims.Email, EmailVerified: claims.EmailVerified}, nil
}
