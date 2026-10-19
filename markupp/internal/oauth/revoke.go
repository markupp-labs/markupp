package oauth

import (
	"context"
	"errors"
)

// errRevokePending marca os métodos da revogação ainda sem implementação.
var errRevokePending = errors.New("oauth: revogação ainda não implementada")

// RevokeToken ainda devolve errRevokePending.
func (s *Service) RevokeToken(context.Context, RevokeRequest) error {
	return errRevokePending
}

// ListAuthorizations ainda devolve errRevokePending.
func (s *Service) ListAuthorizations(context.Context, string) ([]AuthorizationView, error) {
	return nil, errRevokePending
}

// RevokeAuthorization ainda devolve errRevokePending.
func (s *Service) RevokeAuthorization(context.Context, string, string) error {
	return errRevokePending
}
