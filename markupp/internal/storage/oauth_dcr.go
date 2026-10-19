package storage

import (
	"context"
	"errors"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// RegisterDCRClient grava o cliente DCR respeitando o teto de pendentes.
func (r *PostgresOAuthRepository) RegisterDCRClient(_ context.Context, _ oauth.DCRRegistration) error {
	return errors.New("registrar cliente dcr: nao implementado")
}
