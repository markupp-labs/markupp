// Package loginprovider fala com os provedores externos de login (ADR-0054): monta a
// URL de autorização e troca o código pela identidade de quem entrou. Não guarda
// estado da tentativa; quem começa guarda state, verifier e nonce até o callback.
package loginprovider

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"

	"golang.org/x/oauth2"
)

var (
	// ErrLoginRejected diz que o provedor ou a validação recusou a tentativa: código,
	// verifier, nonce ou id_token inválidos. É erro de quem chama, não do servidor.
	ErrLoginRejected = errors.New("login rejected by the provider")
	// ErrProviderUnavailable diz que o provedor não respondeu como esperado.
	ErrProviderUnavailable = errors.New("login provider unavailable")
)

// Settings é um provedor configurado. Sem Issuer, o adaptador é o do GitHub.
type Settings struct {
	ID           string
	Name         string
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Identity é quem o provedor diz que entrou: Subject é o id estável no provedor.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
}

// Attempt é o começo de um login: a URL do provedor e o que precisa voltar no Finish.
type Attempt struct {
	AuthorizationURL string
	State            string
	CodeVerifier     string
	Nonce            string
}

// Provider é um provedor externo de login.
type Provider interface {
	ID() string
	Name() string
	Begin(ctx context.Context) (Attempt, error)
	Finish(ctx context.Context, code, codeVerifier, nonce string) (Identity, error)
}

// newAttemptValues sorteia state, verifier PKCE e nonce de uma tentativa.
func newAttemptValues() Attempt {
	return Attempt{State: rand.Text(), CodeVerifier: oauth2.GenerateVerifier(), Nonce: rand.Text()}
}

// classifyExchangeError separa a falha de transporte, do servidor, da recusa do provedor.
func classifyExchangeError(err error) error {
	var transport *url.Error
	if errors.As(err, &transport) {
		return fmt.Errorf("%w: %w", ErrProviderUnavailable, err)
	}
	return fmt.Errorf("%w: %w", ErrLoginRejected, err)
}
