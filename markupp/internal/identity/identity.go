// Package identity carrega o ator autenticado da requisição HTTP até a transação
// no banco, onde ele define o tenant e o usuário que o RLS enxerga.
package identity

import "context"

// RoleAdmin é o papel que administra usuários e convites do tenant.
const RoleAdmin = "admin"

// Actor é o usuário autenticado, com o tenant e o papel vindos do JWT validado.
type Actor struct {
	UserID   string
	TenantID string
	Role     string
}

// IsAdmin diz se o ator administra o tenant.
func (a Actor) IsAdmin() bool {
	return a.Role == RoleAdmin
}

type actorKey struct{}

// WithActor devolve um contexto que carrega o ator.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFrom devolve o ator do contexto, ou falso quando a requisição não foi autenticada.
func ActorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	return a, ok
}
