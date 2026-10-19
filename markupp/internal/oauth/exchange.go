package oauth

import (
	"context"
	"errors"
)

// errExchangePending marca a troca de token ainda sem implementação (fatia 4 da #54).
var errExchangePending = errors.New("oauth: troca de token ainda não implementada")

// ExchangeToken troca o token do agente pelo token curto da API (RFC 8693).
func (s *Service) ExchangeToken(_ context.Context, _ ExchangeGrant) (ExchangeResponse, error) {
	return ExchangeResponse{}, errExchangePending
}
