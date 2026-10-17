package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// DescribeRequest devolve o que a tela de consentimento mostra sobre um pedido pendente.
// Pedido inexistente, expirado, decidido ou cujo cliente foi apagado sai como
// ErrRequestNotFound embrulhado.
func (s *Service) DescribeRequest(ctx context.Context, requestID, userID string) (RequestView, error) {
	req, err := s.repo.GetPendingRequest(ctx, requestID, s.clock())
	if err != nil {
		return RequestView{}, fmt.Errorf("oauth: buscando o pedido %s: %w", ClientValue(requestID), err)
	}
	client, err := s.repo.GetClient(ctx, req.ClientID)
	if errors.Is(err, ErrClientNotFound) {
		return RequestView{}, clientGoneErr(requestID)
	}
	if err != nil {
		return RequestView{}, fmt.Errorf("oauth: buscando o cliente %s do pedido %s: %w", ClientValue(req.ClientID), ClientValue(requestID), err)
	}
	authorized, err := s.alreadyAuthorized(ctx, userID, req)
	if err != nil {
		return RequestView{}, err
	}
	return RequestView{
		ID: req.ID, ClientID: client.ID, ClientName: client.Name, RedirectHost: redirectHost(req.RedirectURI),
		Scopes: req.Scopes, AlreadyAuthorized: authorized, ExpiresAt: req.ExpiresAt.UTC().Format(time.RFC3339),
	}, nil
}

// clientGoneErr é o erro do pedido cujo cliente foi apagado depois de criado: para a
// conta, o pedido deixou de existir.
func clientGoneErr(requestID string) error {
	return fmt.Errorf("oauth: o cliente do pedido %s foi apagado: %w", ClientValue(requestID), ErrRequestNotFound)
}

// alreadyAuthorized diz se a Autorização ativa da conta para o cliente já cobre todos os
// escopos do pedido.
func (s *Service) alreadyAuthorized(ctx context.Context, userID string, req AuthorizationRequest) (bool, error) {
	active, err := s.repo.ActiveAuthorization(ctx, userID, req.ClientID)
	if errors.Is(err, ErrAuthorizationNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("oauth: buscando a autorização da conta %q para o cliente %s: %w", userID, ClientValue(req.ClientID), err)
	}
	for _, scope := range req.Scopes {
		if !slices.Contains(active.Scopes, scope) {
			return false, nil
		}
	}
	return true, nil
}

// redirectHost devolve o host (com porta, se houver) da redirect_uri já validada.
func redirectHost(redirectURI string) string {
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		return ""
	}
	return parsed.Host
}

// Approve aprova o pedido em nome da conta, emite o código de uso único e devolve o
// redirect ao cliente com code, state e iss.
func (s *Service) Approve(ctx context.Context, requestID, userID string) (string, error) {
	code := s.newToken()
	now := s.clock()
	req, err := s.repo.ApproveRequest(ctx, Approval{
		RequestID: requestID, UserID: userID, CodeHash: auth.HashOpaqueToken(code),
		CodeExpiresAt: now.Add(s.cfg.CodeTTL), DecidedAt: now,
	})
	if errors.Is(err, ErrClientNotFound) {
		return "", clientGoneErr(requestID)
	}
	if err != nil {
		return "", fmt.Errorf("oauth: aprovando o pedido %s: %w", ClientValue(requestID), err)
	}
	return AppendQuery(req.RedirectURI, map[string]string{"code": code, "state": req.State, "iss": s.cfg.Issuer}), nil
}

// Deny recusa o pedido em nome da conta e devolve o redirect ao cliente com access_denied.
func (s *Service) Deny(ctx context.Context, requestID, userID string) (string, error) {
	req, err := s.repo.DenyRequest(ctx, Denial{RequestID: requestID, UserID: userID, DecidedAt: s.clock()})
	if err != nil {
		return "", fmt.Errorf("oauth: recusando o pedido %s: %w", ClientValue(requestID), err)
	}
	return AppendQuery(req.RedirectURI, map[string]string{"error": errAccessDenied, "state": req.State, "iss": s.cfg.Issuer}), nil
}
