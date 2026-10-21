package api

import (
	"context"
	"net/http"
	"slices"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// anyOAuthScope é a política da rota que aceita token OAuth com qualquer escopo.
const anyOAuthScope = "*"

type acceptedOAuthScopeKey struct{}

// acceptOAuthScope declara que a rota aceita token OAuth com o escopo dado, ou com
// qualquer um em anyOAuthScope. Vai antes do RequireAuth, que recusa com
// first_party_only o token OAuth de rota sem essa declaração.
func acceptOAuthScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), acceptedOAuthScopeKey{}, scope)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// allowOAuthActor deixa passar o token de login e o token OAuth que a rota aceita; o
// resto sai com 403, e o escopo faltante com o desafio da RFC 6750.
func allowOAuthActor(w http.ResponseWriter, r *http.Request, actor identity.Actor) bool {
	accepted, _ := r.Context().Value(acceptedOAuthScopeKey{}).(string)
	switch {
	case !actor.FromOAuth, accepted == anyOAuthScope, accepted != "" && grantsScope(actor.OAuthScopes, accepted):
		return true
	case accepted == "":
		writeError(w, "first_party_only", "rota so aceita o token de login", http.StatusForbidden)
	default:
		w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope", scope="`+accepted+`"`)
		writeError(w, "insufficient_scope", "o token nao tem o escopo "+accepted, http.StatusForbidden)
	}
	return false
}

// grantsScope diz se os escopos concedidos cobrem o exigido; notas:escrita também
// satisfaz notas:leitura.
func grantsScope(granted []string, scope string) bool {
	if scope == oauth.ScopeNotesRead && slices.Contains(granted, oauth.ScopeNotesWrite) {
		return true
	}
	return slices.Contains(granted, scope)
}
