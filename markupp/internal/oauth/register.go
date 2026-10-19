package oauth

import (
	"context"
	"errors"
)

// RegisterClient registra um cliente público por DCR (RFC 7591).
func (s *Service) RegisterClient(_ context.Context, _ RegistrationRequest) (RegistrationResponse, error) {
	return RegistrationResponse{}, errors.New("oauth: registro dinamico nao implementado")
}
