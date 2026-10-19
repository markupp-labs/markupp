package oauth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// RefreshToken troca o refresh token por um par novo, rotacionando o refresh (RFC 9700,
// seção 4.14.2). O refresh é consumido antes das conferências: um grant que não bate com
// a Autorização o perde de vez.
func (s *Service) RefreshToken(ctx context.Context, g RefreshGrant) (TokenResponse, error) {
	if err := checkRefreshParams(g); err != nil {
		return TokenResponse{}, err
	}
	now := s.clock()
	grant, err := s.consumeRefresh(ctx, g.RefreshToken, now)
	if err != nil {
		return TokenResponse{}, err
	}
	scopes, err := checkRefreshGrant(g, grant)
	if err != nil {
		return TokenResponse{}, err
	}
	return s.saveRefreshed(ctx, grant, scopes, now)
}

func checkRefreshParams(g RefreshGrant) error {
	if g.RefreshToken == "" {
		return &ProtocolError{Code: errInvalidRequest, Description: "refresh_token ausente; esperado o refresh token emitido por este servidor"}
	}
	if g.ClientID == "" {
		return &ProtocolError{Code: errInvalidRequest, Description: "client_id ausente; esperado o client_id que recebeu o refresh token"}
	}
	return nil
}

// consumeRefresh marca o refresh como rotacionado. Reapresentar um já rotacionado é reuso
// e derruba os tokens da Autorização (RFC 9700, seção 4.14.2).
func (s *Service) consumeRefresh(ctx context.Context, refresh string, now time.Time) (TokenGrant, error) {
	refreshHash := auth.HashOpaqueToken(refresh)
	grant, err := s.repo.ConsumeRefreshToken(ctx, refreshHash, now)
	switch {
	case err == nil:
		return grant, nil
	case errors.Is(err, ErrRefreshReused):
		return TokenGrant{}, s.revokeReusedRefresh(ctx, grant.Authorization, refreshHash, now)
	case errors.Is(err, ErrTokenNotFound), errors.Is(err, ErrAuthorizationNotFound):
		return TokenGrant{}, grantError(revokedRefreshDescription)
	}
	return TokenGrant{}, fmt.Errorf("oauth: consumindo o refresh token: %w", err)
}

// revokedRefreshDescription é a descrição do refresh que não vale mais.
const revokedRefreshDescription = "refresh token invalido, expirado ou revogado; esperado um refresh token emitido por este servidor e ainda valido"

// reusedRefreshDescription é a descrição do refresh já rotacionado reapresentado.
const reusedRefreshDescription = "refresh token ja rotacionado; esperado o refresh token mais recente emitido para o cliente"

// revokeReusedRefresh derruba os tokens da Autorização do refresh reapresentado, com o
// hash dele como gatilho. Expirado também dispara: o que importa é ele seguir sem
// revogação. Se outro reuso do mesmo refresh já revogou, o repositório não revoga nem
// audita de novo.
func (s *Service) revokeReusedRefresh(ctx context.Context, authorization Authorization, refreshHash string, now time.Time) error {
	if authorization.ID == "" {
		return grantError(reusedRefreshDescription)
	}
	event := auditEvent(AuditActionRefreshReuse, AuditResultFailure, authorization, now)
	if err := s.repo.RevokeAuthorizationTokensAudited(ctx, authorization.ID, refreshHash, ExpiredTriggerRevokes, event); err != nil {
		return fmt.Errorf("oauth: revogando os tokens da autorização %q após reuso de refresh token: %w", authorization.ID, err)
	}
	return grantError(reusedRefreshDescription)
}

// checkRefreshGrant confere o grant contra o token consumido e devolve os escopos do
// par novo.
func checkRefreshGrant(g RefreshGrant, grant TokenGrant) ([]string, error) {
	if g.ClientID != grant.Authorization.ClientID {
		return nil, grantError("client_id " + ClientValue(g.ClientID) + " diferente do dono do refresh token; esperado o client_id que recebeu o token")
	}
	if err := checkResource(g.Resource, grant.Token.Resource); err != nil {
		return nil, err
	}
	return narrowScopes(g.Scope, grant.Token.Scopes)
}

// narrowScopes devolve os escopos pedidos no refresh, na ordem de SupportedScopes. Scope
// ausente (ou só com espaços) mantém os do token; escopo além deles é invalid_scope.
func narrowScopes(raw string, granted []string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return granted, nil
	}
	requested := map[string]bool{}
	for _, scope := range strings.Fields(raw) {
		if !slices.Contains(granted, scope) {
			return nil, &ProtocolError{Code: errInvalidScope, Description: "escopo " + ClientValue(scope) + " nao concedido ao refresh token; esperado um subconjunto de " + strings.Join(granted, ", ")}
		}
		requested[scope] = true
	}
	return slices.DeleteFunc(SupportedScopes(), func(scope string) bool { return !requested[scope] }), nil
}

// saveRefreshed grava o par novo com o mesmo resource e audita o refresh, numa transação.
// Uma revogação que entrou depois do consumo derruba o grant sem gravar nada.
func (s *Service) saveRefreshed(ctx context.Context, grant TokenGrant, scopes []string, now time.Time) (TokenResponse, error) {
	base := IssuedToken{AuthorizationID: grant.Authorization.ID, Resource: grant.Token.Resource, Scopes: scopes}
	pair := s.newTokenPair(base, now)
	event := auditEvent(AuditActionRefresh, AuditResultSuccess, grant.Authorization, now)
	err := s.repo.SaveRefreshedTokens(ctx, grant.Token.Hash, pair.tokens, event)
	if errors.Is(err, ErrTokenNotFound) || errors.Is(err, ErrAuthorizationNotFound) {
		return TokenResponse{}, grantError(revokedRefreshDescription)
	}
	if err != nil {
		return TokenResponse{}, fmt.Errorf("oauth: gravando os tokens rotacionados da autorização %q: %w", grant.Authorization.ID, err)
	}
	return pair.response, nil
}

// auditEvent monta o evento da Autorização: ator é a conta, alvo é o cliente.
func auditEvent(action, result string, authorization Authorization, now time.Time) AuditEvent {
	return AuditEvent{Action: action, Result: result, UserID: authorization.UserID, ClientID: authorization.ClientID, OccurredAt: now}
}
