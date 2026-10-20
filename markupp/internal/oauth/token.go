package oauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// ExchangeCode troca o código de autorização por access e refresh token. O código é
// consumido antes das conferências: um grant que não bate com o pedido o perde de vez.
func (s *Service) ExchangeCode(ctx context.Context, g CodeGrant) (TokenResponse, error) {
	now := s.clock()
	redeemed, err := s.redeemCode(ctx, g.Code, now)
	if err != nil {
		return TokenResponse{}, err
	}
	if err := checkGrant(g, redeemed.Request); err != nil {
		return TokenResponse{}, err
	}
	return s.issueTokens(ctx, redeemed, now)
}

// redeemCode consome o código. No reuso, o repositório já revogou os tokens da
// Autorização do pedido (RFC 9700, 4.4.2).
func (s *Service) redeemCode(ctx context.Context, code string, now time.Time) (RedeemedCode, error) {
	redeemed, err := s.repo.RedeemCode(ctx, auth.HashOpaqueToken(code), now, s.cfg.RequestTTL)
	switch {
	case err == nil:
		return redeemed, nil
	case errors.Is(err, ErrCodeAlreadyUsed):
		return RedeemedCode{}, grantError(reusedCodeDescription)
	case errors.Is(err, ErrCodeNotFound), errors.Is(err, ErrAuthorizationNotFound):
		return RedeemedCode{}, grantError(invalidCodeDescription)
	}
	return RedeemedCode{}, fmt.Errorf("oauth: consumindo o código de autorização: %w", err)
}

// invalidCodeDescription é a descrição do código que não vale, ou cuja Autorização foi
// revogada.
const invalidCodeDescription = "codigo de autorizacao invalido ou expirado; esperado um codigo emitido por este servidor e ainda valido"

// reusedCodeDescription é a descrição do código apresentado pela segunda vez.
const reusedCodeDescription = "codigo de autorizacao ja usado; esperado um codigo de uso unico ainda nao trocado"

// checkGrant confere o grant contra o pedido que originou o código.
func checkGrant(g CodeGrant, req AuthorizationRequest) error {
	if g.ClientID != req.ClientID {
		return grantError("client_id " + ClientValue(g.ClientID) + " diferente do pedido; esperado o client_id que pediu o codigo")
	}
	if g.RedirectURI != req.RedirectURI {
		return grantError("redirect_uri " + ClientValue(g.RedirectURI) + " diferente da usada no pedido; esperada a mesma redirect_uri do pedido")
	}
	if !VerifyPKCE(g.CodeVerifier, req.CodeChallenge) {
		return grantError("code_verifier nao confere com o code_challenge do pedido; esperado o verifier que gerou o challenge S256")
	}
	return checkResource(g.Resource, req.Resource)
}

// checkResource confere o resource opcional do grant contra o concedido. Divergente ou
// ilegível é invalid_target (RFC 8707, seção 2.2), não invalid_grant.
func checkResource(raw, granted string) error {
	if raw == "" {
		return nil
	}
	if resource, ok := canonicalize(raw); !ok || resource != granted {
		return &ProtocolError{Code: errInvalidTarget, Description: "resource " + ClientValue(raw) + " diferente do concedido; esperado " + ClientValue(granted)}
	}
	return nil
}

// issueTokens emite e grava o par de tokens com os escopos e o resource do pedido. O
// reuso do código ou a revogação da Autorização que entrou depois do consumo derruba o
// grant sem gravar nada.
func (s *Service) issueTokens(ctx context.Context, redeemed RedeemedCode, now time.Time) (TokenResponse, error) {
	base := IssuedToken{AuthorizationID: redeemed.Authorization.ID, Resource: redeemed.Request.Resource, Scopes: redeemed.Request.Scopes}
	pair := s.newTokenPair(base, now)
	err := s.repo.SaveCodeTokens(ctx, redeemed.Request.ID, pair.tokens)
	switch {
	case err == nil:
		return pair.response, nil
	case errors.Is(err, ErrCodeAlreadyUsed):
		return TokenResponse{}, grantError(reusedCodeDescription)
	case errors.Is(err, ErrAuthorizationNotFound):
		return TokenResponse{}, grantError(invalidCodeDescription)
	}
	return TokenResponse{}, fmt.Errorf("oauth: gravando os tokens da autorização %q: %w", redeemed.Authorization.ID, err)
}

// tokenPair é o par access e refresh recém-gerado: os registros com o hash, para o
// repositório, e a resposta com os tokens em claro, só para o cliente.
type tokenPair struct {
	tokens   []IssuedToken
	response TokenResponse
}

// newTokenPair gera access e refresh com a Autorização, o resource e os escopos de base.
func (s *Service) newTokenPair(base IssuedToken, now time.Time) tokenPair {
	access, refresh := s.newToken(), s.newToken()
	return tokenPair{
		tokens: []IssuedToken{
			withSecret(base, access, TokenKindAccess, now.Add(s.cfg.AccessTTL)),
			withSecret(base, refresh, TokenKindRefresh, now.Add(s.cfg.RefreshTTL)),
		},
		response: TokenResponse{
			AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(s.cfg.AccessTTL / time.Second),
			RefreshToken: refresh, Scope: strings.Join(base.Scopes, " "),
		},
	}
}

func withSecret(base IssuedToken, token, kind string, expiresAt time.Time) IssuedToken {
	base.Hash, base.Kind, base.ExpiresAt = auth.HashOpaqueToken(token), kind, expiresAt
	return base
}

func grantError(description string) error {
	return &ProtocolError{Code: errInvalidGrant, Description: description}
}
