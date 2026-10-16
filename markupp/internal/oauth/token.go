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

// redeemCode consome o código. Reuso revoga os tokens da Autorização (RFC 9700, 4.4.2).
func (s *Service) redeemCode(ctx context.Context, code string, now time.Time) (RedeemedCode, error) {
	redeemed, err := s.repo.RedeemCode(ctx, auth.HashOpaqueToken(code), now)
	switch {
	case err == nil:
		return redeemed, nil
	case errors.Is(err, ErrCodeAlreadyUsed):
		return RedeemedCode{}, s.revokeReusedCode(ctx, redeemed.Authorization.ID)
	case errors.Is(err, ErrCodeNotFound), errors.Is(err, ErrAuthorizationNotFound):
		return RedeemedCode{}, grantError("codigo de autorizacao invalido ou expirado")
	}
	return RedeemedCode{}, fmt.Errorf("oauth: consumindo o código de autorização: %w", err)
}

func (s *Service) revokeReusedCode(ctx context.Context, authorizationID string) error {
	if authorizationID == "" {
		return grantError("codigo de autorizacao ja usado")
	}
	if err := s.repo.RevokeAuthorizationTokens(ctx, authorizationID); err != nil {
		return fmt.Errorf("oauth: revogando os tokens da autorização %q após reuso de código: %w", authorizationID, err)
	}
	return grantError("codigo de autorizacao ja usado")
}

// checkGrant confere o grant contra o pedido que originou o código.
func checkGrant(g CodeGrant, req AuthorizationRequest) error {
	if g.ClientID != req.ClientID {
		return grantError("client_id " + describeValue(g.ClientID) + " diferente do pedido")
	}
	if g.RedirectURI != req.RedirectURI {
		return grantError("redirect_uri " + describeValue(g.RedirectURI) + " diferente da usada no pedido")
	}
	if !VerifyPKCE(g.CodeVerifier, req.CodeChallenge) {
		return grantError("code_verifier nao confere com o code_challenge do pedido")
	}
	if g.Resource == "" {
		return nil
	}
	if resource, ok := canonicalize(g.Resource); !ok || resource != req.Resource {
		return grantError("resource " + describeValue(g.Resource) + " diferente do pedido")
	}
	return nil
}

// issueTokens emite e grava o par de tokens com os escopos e o resource do pedido.
func (s *Service) issueTokens(ctx context.Context, redeemed RedeemedCode, now time.Time) (TokenResponse, error) {
	access, refresh := s.newToken(), s.newToken()
	tokens := []IssuedToken{
		issuedToken(access, TokenKindAccess, redeemed, now.Add(s.cfg.AccessTTL)),
		issuedToken(refresh, TokenKindRefresh, redeemed, now.Add(s.cfg.RefreshTTL)),
	}
	if err := s.repo.SaveTokens(ctx, tokens); err != nil {
		return TokenResponse{}, fmt.Errorf("oauth: gravando os tokens da autorização %q: %w", redeemed.Authorization.ID, err)
	}
	return TokenResponse{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(s.cfg.AccessTTL / time.Second),
		RefreshToken: refresh, Scope: strings.Join(redeemed.Request.Scopes, " "),
	}, nil
}

func issuedToken(token, kind string, redeemed RedeemedCode, expiresAt time.Time) IssuedToken {
	return IssuedToken{
		Hash: auth.HashOpaqueToken(token), AuthorizationID: redeemed.Authorization.ID, Kind: kind,
		Resource: redeemed.Request.Resource, Scopes: redeemed.Request.Scopes, ExpiresAt: expiresAt,
	}
}

func grantError(description string) error {
	return &ProtocolError{Code: errInvalidGrant, Description: description}
}
