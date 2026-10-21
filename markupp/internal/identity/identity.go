// Package identity carrega o ator autenticado da requisição HTTP até a transação
// no banco, onde ele define o tenant e o usuário que o RLS enxerga.
package identity

import "context"

// Papéis do usuário dentro de um tenant, do maior para o menor.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleViewer = "viewer"
)

// Actor é o usuário do JWT validado, com o tenant em que a requisição age e o papel
// dele lá, lidos do vínculo no banco. FromOAuth marca o token entregue pela troca
// OAuth, que só alcança o que OAuthScopes permite; token de login tem acesso inteiro.
type Actor struct {
	UserID      string
	TenantID    string
	Role        string
	FromOAuth   bool
	OAuthScopes []string
}

var roleRanks = map[string]int{RoleOwner: 4, RoleAdmin: 3, RoleMember: 2, RoleViewer: 1}

// RoleRank é o posto do papel, maior para quem pode mais. Papel desconhecido vale zero.
func RoleRank(role string) int {
	return roleRanks[role]
}

// IsAdmin diz se o ator administra o tenant: owner ou admin.
func (a Actor) IsAdmin() bool {
	return a.Role == RoleOwner || a.Role == RoleAdmin
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
