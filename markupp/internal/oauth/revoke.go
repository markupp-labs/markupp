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
// erro, para não revelar ao cliente se o token existe. Para o token de outro cliente, é
// um desvio intencional da RFC 7009, seção 2.1, que manda recusar o pedido (ADR-0046).
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
		return fmt.Errorf("oauth: find token to revoke: %w", err)
	}
	now := s.clock()
	if r.ClientID != grant.Authorization.ClientID || !liveGrant(grant, now) {
		return nil
	}
	return s.revokeTokens(ctx, grant.Authorization, tokenHash, now)
}

func checkRevokeParams(r RevokeRequest) error {
	if r.Token == "" {
		return &ProtocolError{Code: errInvalidRequest, Description: "token missing; expected the access or refresh token to revoke"}
	}
	if r.ClientID == "" {
		return &ProtocolError{Code: errInvalidRequest, Description: "client_id missing; expected the client_id that received the token"}
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
	if err := s.repo.RevokeAuthorizationTokensAudited(ctx, authorization.ID, tokenHash, ExpiredTriggerIgnored, event); err != nil {
		return fmt.Errorf("oauth: revoke tokens of authorization %q: %w", authorization.ID, err)
	}
	return nil
}

// ListAuthorizations devolve as Autorizações ativas da conta, nunca nil, nem nos escopos.
func (s *Service) ListAuthorizations(ctx context.Context, userID string) ([]AuthorizationView, error) {
	summaries, err := s.repo.ListActiveAuthorizations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("oauth: list authorizations of account %q: %w", userID, err)
	}
	views := make([]AuthorizationView, 0, len(summaries))
	for _, summary := range summaries {
		views = append(views, AuthorizationView{
			ID: summary.ID, ClientID: summary.ClientID, ClientName: summary.ClientName, Scopes: append([]string{}, summary.Scopes...),
			CreatedAt: summary.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: summary.UpdatedAt.UTC().Format(time.RFC3339),
			IP: summary.Origin.IP, UserAgent: summary.Origin.UserAgent, LastUsedAt: formatLastUse(summary.LastUsedAt),
		})
	}
	return views, nil
}

// RevokeAuthorization revoga a Autorização da conta e os tokens dela. O repositório
// preenche o cliente alvo do evento na transação; Autorização inexistente, revogada ou
// de outra conta sai como ErrAuthorizationNotFound.
func (s *Service) RevokeAuthorization(ctx context.Context, userID, authorizationID string) error {
	return s.RevokeAuthorizationAs(ctx, userID, userID, authorizationID)
}

// RevokeAuthorizationAs revoga a Autorização de userID a pedido de actorID, que é a
// própria conta ou o administrador da instância (ADR-0061), auditado como autor.
func (s *Service) RevokeAuthorizationAs(ctx context.Context, actorID, userID, authorizationID string) error {
	event := AuditEvent{Action: AuditActionRevoke, Result: AuditResultSuccess, UserID: actorID, OccurredAt: s.clock()}
	if err := s.repo.RevokeAuthorization(ctx, userID, authorizationID, event); err != nil {
		return fmt.Errorf("oauth: revoke authorization %s of account %q: %w", ClientValue(authorizationID), userID, err)
	}
	return nil
}

// formatLastUse escreve o último uso em RFC 3339 UTC, vazio quando nunca houve.
func formatLastUse(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

// touchAuthorization registra o uso da Autorização com a origem do pedido.
func (s *Service) touchAuthorization(ctx context.Context, authorizationID string, now time.Time) error {
	if err := s.repo.TouchAuthorization(ctx, authorizationID, auth.OriginFrom(ctx), now); err != nil {
		return fmt.Errorf("oauth: record use of authorization %q: %w", authorizationID, err)
	}
	return nil
}

// blockedAccountDescription é a recusa do token à conta bloqueada (ADR-0061).
const blockedAccountDescription = "account blocked by the instance administrator; expected an active account"

// checkAccountActive recusa com invalid_grant a conta bloqueada: nenhum token novo sai
// para ela, nem pela troca do código, nem pelo refresh, nem pela troca do MCP.
func (s *Service) checkAccountActive(ctx context.Context, userID string) error {
	blocked, err := s.repo.AccountBlocked(ctx, userID)
	if err != nil {
		return fmt.Errorf("oauth: check account %q: %w", userID, err)
	}
	if blocked {
		return grantError(blockedAccountDescription)
	}
	return nil
}
