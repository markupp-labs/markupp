package oauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// ExchangeToken troca o access token do agente pelo token curto da API (RFC 8693). Só o
// resource autenticado com o próprio segredo troca, e só tokens emitidos para ele.
func (s *Service) ExchangeToken(ctx context.Context, g ExchangeGrant) (ExchangeResponse, error) {
	resource, err := s.authenticateResource(g)
	if err != nil {
		return ExchangeResponse{}, err
	}
	if err := s.checkExchangeParams(g); err != nil {
		return ExchangeResponse{}, err
	}
	now := s.clock()
	grant, err := s.findSubjectToken(ctx, g.SubjectToken, resource, now)
	if err != nil {
		return ExchangeResponse{}, err
	}
	return s.issueExchanged(ctx, grant, now)
}

// authenticateResource confere o resource pelo HTTP Basic e devolve a URL canônica dele.
// O erro não cita o usuário recebido, que pode ser o segredo num par invertido.
func (s *Service) authenticateResource(g ExchangeGrant) (string, error) {
	resource, ok := canonicalize(g.ResourceID)
	secret, known := s.cfg.ResourceSecrets[resource]
	if !sameSecret(g.ResourceSecret, secret) || !ok || !known {
		return "", errResourceUnauthenticated()
	}
	return resource, nil
}

// AuthenticatesResource diz se o par é a URL de um resource configurado e o segredo
// dele, com a mesma conferência da troca de token. A leitura pública usa isso para
// confiar no IP do agente que o servidor MCP repassa (ADR-0051).
func (s *Service) AuthenticatesResource(resourceID, secret string) bool {
	_, err := s.authenticateResource(ExchangeGrant{ResourceID: resourceID, ResourceSecret: secret})
	return err == nil
}

// sameSecret compara os resumos SHA-256 dos dois segredos em tempo constante. Os resumos
// têm sempre o mesmo tamanho, então nem o tamanho do segredo nem um resource desconhecido
// (segredo vazio) mudam o tempo da comparação.
func sameSecret(received, configured string) bool {
	receivedSum := sha256.Sum256([]byte(received))
	configuredSum := sha256.Sum256([]byte(configured))
	return subtle.ConstantTimeCompare(receivedSum[:], configuredSum[:]) == 1
}

func errResourceUnauthenticated() error {
	return &ProtocolError{Code: errInvalidClient, Description: "resource nao autenticado; esperado HTTP Basic com a url de um resource configurado e o segredo dele"}
}

// checkExchangeParams confere os tipos de token e o alvo da troca, que é a API deste issuer.
func (s *Service) checkExchangeParams(g ExchangeGrant) error {
	if g.SubjectToken == "" {
		return &ProtocolError{Code: errInvalidRequest, Description: "subject_token ausente; esperado o access token recebido pelo resource"}
	}
	if g.SubjectTokenType != TokenTypeAccessToken {
		return &ProtocolError{Code: errInvalidRequest, Description: "subject_token_type " + ClientValue(g.SubjectTokenType) + " nao suportado; esperado " + TokenTypeAccessToken}
	}
	if g.RequestedTokenType != "" && g.RequestedTokenType != TokenTypeAccessToken {
		return &ProtocolError{Code: errInvalidRequest, Description: "requested_token_type " + ClientValue(g.RequestedTokenType) + " nao suportado; esperado " + TokenTypeAccessToken + " ou ausente"}
	}
	if err := s.checkExchangeTarget("audience", g.Audience); err != nil {
		return err
	}
	return s.checkExchangeTarget("resource", g.Resource)
}

// checkExchangeTarget aceita o alvo ausente ou com a forma canônica do issuer.
func (s *Service) checkExchangeTarget(name, raw string) error {
	if raw == "" {
		return nil
	}
	if target, ok := canonicalize(raw); !ok || target != s.cfg.Issuer {
		return &ProtocolError{Code: errInvalidTarget, Description: name + " " + ClientValue(raw) + " diferente da API; esperado " + ClientValue(s.cfg.Issuer) + " ou ausente"}
	}
	return nil
}

// invalidSubjectDescription é a descrição do subject_token que não vale para a troca.
const invalidSubjectDescription = "subject_token invalido, expirado, revogado ou de outro resource; esperado um access token vigente emitido para o resource autenticado"

// findSubjectToken busca o access token pelo hash e confere que ele vale em now para o
// resource autenticado.
func (s *Service) findSubjectToken(ctx context.Context, subject, resource string, now time.Time) (TokenGrant, error) {
	grant, err := s.repo.FindToken(ctx, auth.HashOpaqueToken(subject))
	if errors.Is(err, ErrTokenNotFound) {
		return TokenGrant{}, grantError(invalidSubjectDescription)
	}
	if err != nil {
		return TokenGrant{}, fmt.Errorf("oauth: buscando o subject_token da troca: %w", err)
	}
	if !exchangeable(grant, resource, now) {
		return TokenGrant{}, grantError(invalidSubjectDescription)
	}
	return grant, nil
}

// exchangeable diz se o token é um access token vigente do resource e com escopos: sem
// eles, o token emitido sairia sem a claim scope e valeria como o de login.
func exchangeable(grant TokenGrant, resource string, now time.Time) bool {
	hasScopes := strings.TrimSpace(strings.Join(grant.Token.Scopes, "")) != ""
	return grant.Token.Kind == TokenKindAccess && grant.Token.Resource == resource && liveGrant(grant, now) && hasScopes
}

// issueExchanged emite o token da API com os escopos do access token e audita a troca.
func (s *Service) issueExchanged(ctx context.Context, grant TokenGrant, now time.Time) (ExchangeResponse, error) {
	token, err := s.scoped.IssueScopedToken(grant.Authorization.UserID, grant.Token.Scopes, s.cfg.ExchangeTTL)
	if err != nil {
		return ExchangeResponse{}, fmt.Errorf("oauth: emitindo o token da troca da autorização %q: %w", grant.Authorization.ID, err)
	}
	event := auditEvent(AuditActionExchange, AuditResultSuccess, grant.Authorization, now)
	if err := s.repo.RecordAuditEvent(ctx, event); err != nil {
		return ExchangeResponse{}, fmt.Errorf("oauth: auditando a troca da autorização %q: %w", grant.Authorization.ID, err)
	}
	return ExchangeResponse{
		AccessToken: token, IssuedTokenType: TokenTypeAccessToken, TokenType: "Bearer",
		ExpiresIn: int64(s.cfg.ExchangeTTL / time.Second), Scope: strings.Join(grant.Token.Scopes, " "),
	}, nil
}
