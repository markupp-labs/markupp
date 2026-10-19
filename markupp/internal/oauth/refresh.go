package oauth

import (
	"context"
	"errors"
)

// errRefreshPending marca os métodos do refresh e da revogação ainda sem implementação.
var errRefreshPending = errors.New("oauth: refresh e revogação ainda não implementados")

// RefreshToken ainda devolve errRefreshPending.
func (s *Service) RefreshToken(context.Context, RefreshGrant) (TokenResponse, error) {
	return TokenResponse{}, errRefreshPending
}

// RevokeToken ainda devolve errRefreshPending.
func (s *Service) RevokeToken(context.Context, RevokeRequest) error {
	return errRefreshPending
}

// ListAuthorizations ainda devolve errRefreshPending.
func (s *Service) ListAuthorizations(context.Context, string) ([]AuthorizationView, error) {
	return nil, errRefreshPending
}

// RevokeAuthorization ainda devolve errRefreshPending.
func (s *Service) RevokeAuthorization(context.Context, string, string) error {
	return errRefreshPending
}
