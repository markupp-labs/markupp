package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// mountOAuthAuthorizationRoutes monta as Autorizações da conta só sob /v1, sem o alias
// obsoleto sem prefixo: exigem login, mas não tenant, porque a Autorização é da conta.
func mountOAuthAuthorizationRoutes(r chi.Router, svc OAuthService, tokenValidator TokenValidator) {
	h := &oauthHandler{svc: svc}
	r.Route(currentVersionPrefix+"/me/authorizations", func(g chi.Router) {
		g.Use(RequireAuth(tokenValidator))
		g.Get("/", h.listAuthorizations)
		g.Delete("/{id}", h.revokeAuthorization)
	})
}

func (h *oauthHandler) listAuthorizations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, _ := identity.ActorFrom(r.Context())
	views, err := h.svc.ListAuthorizations(r.Context(), actor.UserID)
	if err != nil {
		writeOAuthInternal(w, "listar autorizacoes oauth", err)
		return
	}
	writeJSON(w, http.StatusOK, authorizationListFrom(views))
}

// authorizationListFrom troca lista e escopos nulos por vazios, para o JSON sempre
// trazer arrays.
func authorizationListFrom(views []oauth.AuthorizationView) []oauth.AuthorizationView {
	out := make([]oauth.AuthorizationView, 0, len(views))
	for _, v := range views {
		if v.Scopes == nil {
			v.Scopes = []string{}
		}
		out = append(out, v)
	}
	return out
}

// revokeAuthorization não distingue Autorização inexistente, já revogada ou de outra
// conta.
func (h *oauthHandler) revokeAuthorization(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, _ := identity.ActorFrom(r.Context())
	err := h.svc.RevokeAuthorization(r.Context(), actor.UserID, chi.URLParam(r, "id"))
	if errors.Is(err, oauth.ErrAuthorizationNotFound) {
		writeError(w, "authorization_not_found", "autorizacao inexistente ou ja revogada", http.StatusNotFound)
		return
	}
	if err != nil {
		writeOAuthInternal(w, "revogar autorizacao oauth", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
