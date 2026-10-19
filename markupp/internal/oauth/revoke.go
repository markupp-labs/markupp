package oauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// RevokeToken atende POST /oauth/revoke (RFC 7009): derruba todos os tokens da
// Autorização do token, que continua ativa. Token desconhecido ou de outro cliente não
// revoga nada e não é erro, para não revelar ao cliente se o token existe.
func (s *Service) RevokeToken(ctx context.Context, r RevokeRequest) error {
	if r.Token == "" {
		return &ProtocolError{Code: errInvalidRequest, Description: "token ausente; esperado o access ou refresh token a revogar"}
	}
	grant, err := s.repo.FindToken(ctx, auth.HashOpaqueToken(r.Token))
	if errors.Is(err, ErrTokenNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("oauth: buscando o token a revogar: %w", err)
	}
	if r.ClientID != "" && r.ClientID != grant.Authorization.ClientID {
		return nil
	}
	return s.revokeTokens(ctx, grant.Authorization)
}

func (s *Service) revokeTokens(ctx context.Context, authorization Authorization) error {
	event := auditEvent(AuditActionRevoke, AuditResultSuccess, authorization, s.clock())
	if err := s.repo.RevokeAuthorizationTokensAudited(ctx, authorization.ID, event); err != nil {
		return fmt.Errorf("oauth: revogando os tokens da autorização %q: %w", authorization.ID, err)
	}
	return nil
}

// ListAuthorizations devolve as Autorizações ativas da conta, nunca nil.
func (s *Service) ListAuthorizations(ctx context.Context, userID string) ([]AuthorizationView, error) {
	summaries, err := s.repo.ListActiveAuthorizations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("oauth: listando as autorizações da conta %q: %w", userID, err)
	}
	views := make([]AuthorizationView, 0, len(summaries))
	for _, summary := range summaries {
		views = append(views, AuthorizationView{
			ID: summary.ID, ClientID: summary.ClientID, ClientName: summary.ClientName, Scopes: summary.Scopes,
			CreatedAt: summary.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: summary.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	return views, nil
}

// RevokeAuthorization revoga a Autorização da conta e os tokens dela. A Autorização é
// lida antes entre as ativas da conta para o evento levar o cliente como alvo; uma que
// não está lá, ou que é revogada no meio do caminho, sai como ErrAuthorizationNotFound.
func (s *Service) RevokeAuthorization(ctx context.Context, userID, authorizationID string) error {
	authorization, err := s.activeAuthorizationByID(ctx, userID, authorizationID)
	if err != nil {
		return err
	}
	event := auditEvent(AuditActionRevoke, AuditResultSuccess, authorization, s.clock())
	if err := s.repo.RevokeAuthorization(ctx, userID, authorization.ID, event); err != nil {
		return fmt.Errorf("oauth: revogando a autorização %s da conta %q: %w", ClientValue(authorizationID), userID, err)
	}
	return nil
}

// activeAuthorizationByID procura a Autorização entre as ativas da conta.
// ponytail: varre a lista da conta, que tem poucas Autorizações; um GetAuthorization no
// Repository resolve se ela crescer.
func (s *Service) activeAuthorizationByID(ctx context.Context, userID, authorizationID string) (Authorization, error) {
	if authorizationID == "" {
		return Authorization{}, fmt.Errorf("oauth: id da autorização ausente; esperado o id de uma autorização ativa da conta: %w", ErrAuthorizationNotFound)
	}
	summaries, err := s.repo.ListActiveAuthorizations(ctx, userID)
	if err != nil {
		return Authorization{}, fmt.Errorf("oauth: listando as autorizações da conta %q: %w", userID, err)
	}
	for _, summary := range summaries {
		if summary.ID == authorizationID {
			return summary.Authorization, nil
		}
	}
	return Authorization{}, fmt.Errorf("oauth: autorização %s fora das ativas da conta %q; esperado o id de uma autorização ativa dela: %w", ClientValue(authorizationID), userID, ErrAuthorizationNotFound)
}
