package oauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// RevokeToken atende POST /oauth/revoke (RFC 7009): derruba todos os tokens da
// Autorização do token, que continua ativa. Token desconhecido, de outro cliente ou que
// já não vale (revogado, expirado ou de Autorização revogada) não revoga nada e não é
// erro, para não revelar ao cliente se o token existe.
func (s *Service) RevokeToken(ctx context.Context, r RevokeRequest) error {
	if err := checkRevokeParams(r); err != nil {
		return err
	}
	tokenHash := auth.HashOpaqueToken(r.Token)
	grant, err := s.repo.FindToken(ctx, tokenHash)
	if errors.Is(err, ErrTokenNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("oauth: buscando o token a revogar: %w", err)
	}
	now := s.clock()
	if r.ClientID != grant.Authorization.ClientID || !liveGrant(grant, now) {
		return nil
	}
	return s.revokeTokens(ctx, grant.Authorization, tokenHash, now)
}

func checkRevokeParams(r RevokeRequest) error {
	if r.Token == "" {
		return &ProtocolError{Code: errInvalidRequest, Description: "token ausente; esperado o access ou refresh token a revogar"}
	}
	if r.ClientID == "" {
		return &ProtocolError{Code: errInvalidRequest, Description: "client_id ausente; esperado o client_id que recebeu o token"}
	}
	return nil
}

// liveGrant diz se o token ainda vale em now, numa Autorização ativa.
func liveGrant(grant TokenGrant, now time.Time) bool {
	return !grant.Token.Revoked && grant.Token.ExpiresAt.After(now) && !grant.Authorization.Revoked
}

// revokeTokens derruba os tokens da Autorização do token lido fora da transação, com o
// hash dele como gatilho. O repositório confere de novo, já com a trava, que o token
// segue vigente: se outra revogação entrou no meio, não revoga nem audita de novo.
func (s *Service) revokeTokens(ctx context.Context, authorization Authorization, tokenHash string, now time.Time) error {
	event := auditEvent(AuditActionRevoke, AuditResultSuccess, authorization, now)
	if _, err := s.repo.RevokeAuthorizationTokensAudited(ctx, authorization.ID, tokenHash, true, event); err != nil {
		return fmt.Errorf("oauth: revogando os tokens da autorização %q: %w", authorization.ID, err)
	}
	return nil
}

// ListAuthorizations devolve as Autorizações ativas da conta, nunca nil, nem nos escopos.
func (s *Service) ListAuthorizations(ctx context.Context, userID string) ([]AuthorizationView, error) {
	summaries, err := s.repo.ListActiveAuthorizations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("oauth: listando as autorizações da conta %q: %w", userID, err)
	}
	views := make([]AuthorizationView, 0, len(summaries))
	for _, summary := range summaries {
		views = append(views, AuthorizationView{
			ID: summary.ID, ClientID: summary.ClientID, ClientName: summary.ClientName, Scopes: append([]string{}, summary.Scopes...),
			CreatedAt: summary.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: summary.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	return views, nil
}

// RevokeAuthorization revoga a Autorização da conta e os tokens dela. O repositório
// preenche o cliente alvo do evento na transação; Autorização inexistente, revogada ou
// de outra conta sai como ErrAuthorizationNotFound.
func (s *Service) RevokeAuthorization(ctx context.Context, userID, authorizationID string) error {
	event := AuditEvent{Action: AuditActionRevoke, Result: AuditResultSuccess, UserID: userID, OccurredAt: s.clock()}
	if err := s.repo.RevokeAuthorization(ctx, userID, authorizationID, event); err != nil {
		return fmt.Errorf("oauth: revogando a autorização %s da conta %q: %w", ClientValue(authorizationID), userID, err)
	}
	return nil
}
