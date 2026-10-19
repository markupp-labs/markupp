package storage

import (
	"context"
	"errors"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// errOAuthRefreshPending marca os métodos do refresh e da revogação ainda sem
// implementação; nenhum caminho de produção os chama antes da fatia 3 da #54.
var errOAuthRefreshPending = errors.New("storage: refresh e revogação oauth ainda não implementados")

// ConsumeRefreshToken ainda devolve errOAuthRefreshPending.
func (r *PostgresOAuthRepository) ConsumeRefreshToken(context.Context, string, time.Time) (oauth.TokenGrant, error) {
	return oauth.TokenGrant{}, errOAuthRefreshPending
}

// SaveRefreshedTokens ainda devolve errOAuthRefreshPending.
func (r *PostgresOAuthRepository) SaveRefreshedTokens(context.Context, []oauth.IssuedToken, oauth.AuditEvent) error {
	return errOAuthRefreshPending
}

// RevokeAuthorizationTokensAudited ainda devolve errOAuthRefreshPending.
func (r *PostgresOAuthRepository) RevokeAuthorizationTokensAudited(context.Context, string, oauth.AuditEvent) error {
	return errOAuthRefreshPending
}

// FindToken ainda devolve errOAuthRefreshPending.
func (r *PostgresOAuthRepository) FindToken(context.Context, string) (oauth.TokenGrant, error) {
	return oauth.TokenGrant{}, errOAuthRefreshPending
}

// ListActiveAuthorizations ainda devolve errOAuthRefreshPending.
func (r *PostgresOAuthRepository) ListActiveAuthorizations(context.Context, string) ([]oauth.AuthorizationSummary, error) {
	return nil, errOAuthRefreshPending
}

// RevokeAuthorization ainda devolve errOAuthRefreshPending.
func (r *PostgresOAuthRepository) RevokeAuthorization(context.Context, string, string, oauth.AuditEvent) error {
	return errOAuthRefreshPending
}
