// Package tenant define os tipos e helpers de contexto para isolamento multi-tenant.
package tenant

import "context"

type contextKey struct{}

var tenantKey = contextKey{}

// Context carrega a identidade de tenant e usuario associada a requisicao.
type Context struct {
	TenantID string
	UserID   string
	Role     string
}

// WithContext anexa o Context do tenant ao context.Context.
func WithContext(ctx context.Context, tc Context) context.Context {
	return context.WithValue(ctx, tenantKey, tc)
}

// FromContext recupera o Context do tenant se presente no context.Context.
func FromContext(ctx context.Context) (Context, bool) {
	tc, ok := ctx.Value(tenantKey).(Context)
	return tc, ok
}

// IDFromContext devolve o ID do tenant a partir do contexto ou o fallback fornecido.
func IDFromContext(ctx context.Context, fallback string) string {
	if tc, ok := FromContext(ctx); ok && tc.TenantID != "" {
		return tc.TenantID
	}
	return fallback
}
