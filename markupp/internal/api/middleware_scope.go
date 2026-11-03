package api

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// anyOAuthScope é a política da rota que aceita token OAuth com qualquer escopo.
const anyOAuthScope = "*"

type acceptedOAuthScopesKey struct{}

// acceptOAuthScope declara que a rota aceita token OAuth com qualquer um dos escopos
// dados, ou com qualquer escopo em anyOAuthScope. Vai antes do RequireAuth, que recusa
// com first_party_only o token OAuth de rota sem essa declaração ou sem escopo algum.
func acceptOAuthScope(scopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), acceptedOAuthScopesKey{}, scopes)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// allowOAuthActor deixa passar o token de login e o token OAuth que a rota aceita; o
// resto sai com 403, e o escopo faltante com o desafio da RFC 6750.
func allowOAuthActor(w http.ResponseWriter, r *http.Request, actor identity.Actor) bool {
	accepted, _ := r.Context().Value(acceptedOAuthScopesKey{}).([]string)
	switch {
	case !actor.FromOAuth, slices.Contains(accepted, anyOAuthScope), grantsAnyScope(actor.OAuthScopes, accepted):
		return true
	case len(accepted) == 0:
		writeError(w, "first_party_only", "rota so aceita o token de login", http.StatusForbidden)
	default:
		w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope", scope="`+strings.Join(accepted, " ")+`"`)
		writeError(w, "insufficient_scope", "o token nao tem o escopo "+strings.Join(accepted, " ou "), http.StatusForbidden)
	}
	return false
}

// grantsAnyScope diz se os escopos concedidos cobrem algum dos aceitos.
func grantsAnyScope(granted, accepted []string) bool {
	return slices.ContainsFunc(accepted, func(scope string) bool { return grantsScope(granted, scope) })
}

// grantsScope diz se os escopos concedidos cobrem o exigido; notas:escrita também
// satisfaz notas:leitura.
func grantsScope(granted []string, scope string) bool {
	if scope == oauth.ScopeNotesRead && slices.Contains(granted, oauth.ScopeNotesWrite) {
		return true
	}
	return slices.Contains(granted, scope)
}
