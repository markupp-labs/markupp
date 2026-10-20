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
// seção 4.14.2). Scope e resource são conferidos antes de consumir o refresh, que segue
// valendo depois de um invalid_scope ou invalid_target; client_id de outro cliente o
// perde de vez.
func (s *Service) RefreshToken(ctx context.Context, g RefreshGrant) (TokenResponse, error) {
	if err := checkRefreshParams(g); err != nil {
		return TokenResponse{}, err
	}
	now := s.clock()
	if err := s.precheckRefresh(ctx, g, now); err != nil {
		return TokenResponse{}, err
	}
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

// precheckRefresh lê o refresh sem consumir e, se ele ainda vale para o client_id do
// grant, confere scope e resource. Refresh que não vale (desconhecido, rotacionado,
// revogado, expirado ou de outro cliente) fica para o consumo, que trata o reuso e queima
// o refresh de outro cliente.
func (s *Service) precheckRefresh(ctx context.Context, g RefreshGrant, now time.Time) error {
	grant, err := s.repo.FindToken(ctx, auth.HashOpaqueToken(g.RefreshToken))
	if errors.Is(err, ErrTokenNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("oauth: lendo o refresh token antes de consumir: %w", err)
	}
	if !unspentRefresh(grant, g.ClientID, now) {
		return nil
	}
	_, err = checkRefreshGrant(g, grant)
	return err
}

// unspentRefresh diz se o token é um refresh do cliente que ainda vale e não foi
// rotacionado.
func unspentRefresh(grant TokenGrant, clientID string, now time.Time) bool {
	return grant.Token.Kind == TokenKindRefresh && !grant.Token.Rotated &&
		grant.Authorization.ClientID == clientID && liveGrant(grant, now)
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

// checkRefreshGrant confere o grant contra o refresh, lido ou consumido, e devolve os
// escopos do access novo.
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
// O access leva os escopos pedidos e o refresh mantém os do refresh consumido (RFC 6749,
// seção 6). Uma revogação que entrou depois do consumo derruba o grant sem gravar nada.
func (s *Service) saveRefreshed(ctx context.Context, grant TokenGrant, scopes []string, now time.Time) (TokenResponse, error) {
	base := IssuedToken{AuthorizationID: grant.Authorization.ID, Resource: grant.Token.Resource, Scopes: scopes}
	pair := s.newTokenPair(base, grant.Token.Scopes, now)
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
