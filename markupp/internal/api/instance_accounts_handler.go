package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

const maxAccountPageSize = 200

// InstanceAccountsService é a gestão das contas pelo administrador da instância (ADR-0061).
type InstanceAccountsService interface {
	List(ctx context.Context, q auth.AccountQuery) ([]auth.AccountSummary, error)
	Get(ctx context.Context, id string) (auth.AccountDetail, error)
	SetBlocked(ctx context.Context, actorID, id string, blocked bool) error
	Delete(ctx context.Context, actorID, id string) error
}

// AccountAuthorizationService lista e revoga os apps autorizados de uma conta, com o
// administrador da instância como autor.
type AccountAuthorizationService interface {
	ListAuthorizations(ctx context.Context, userID string) ([]oauth.AuthorizationView, error)
	RevokeAuthorizationAs(ctx context.Context, actorID, userID, authorizationID string) error
}

type instanceAccountsHandler struct {
	accounts InstanceAccountsService
	sessions SessionService
	apps     AccountAuthorizationService
}

// mountInstanceAccountRoutes monta /instance/accounts com o escopo de administração e só
// para quem administra a instância. Sessões e apps entram quando os serviços existem.
func mountInstanceAccountRoutes(r chi.Router, h *instanceAccountsHandler, accounts AuthService, admins instance.Admins, validator TokenValidator) {
	g := r.With(acceptOAuthScope(oauth.ScopeAdmin), RequireAuth(validator), requireInstanceAdmin(accounts, admins), noStore)
	g.Get("/instance/accounts", h.list)
	g.Get("/instance/accounts/{id}", h.get)
	g.Patch("/instance/accounts/{id}", h.patch)
	g.Delete("/instance/accounts/{id}", h.delete)
	if h.sessions != nil {
		g.Get("/instance/accounts/{id}/sessions", h.listSessions)
		g.Delete("/instance/accounts/{id}/sessions/{session_id}", h.endSession)
	}
	if h.apps != nil {
		g.Get("/instance/accounts/{id}/authorizations", h.listAuthorizations)
		g.Delete("/instance/accounts/{id}/authorizations/{authorization_id}", h.revokeAuthorization)
	}
}

// noStore marca a resposta como não cacheável: ela traz dados de contas.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (h *instanceAccountsHandler) list(w http.ResponseWriter, r *http.Request) {
	q, err := accountQueryFrom(r)
	if err != nil {
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
		return
	}
	accounts, err := h.accounts.List(r.Context(), q)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	if accounts == nil {
		accounts = []auth.AccountSummary{}
	}
	writeJSON(w, http.StatusOK, accounts)
}

func accountQueryFrom(r *http.Request) (auth.AccountQuery, error) {
	qr := &queryReader{values: r.URL.Query()}
	q := auth.AccountQuery{
		Email: qr.values.Get("q"), Status: qr.values.Get("status"),
		Offset: qr.intBetween("offset", 0, 0, maxOffset),
		Limit:  qr.intBetween("limit", 0, 1, maxAccountPageSize),
	}
	return q, qr.err
}

func (h *instanceAccountsHandler) get(w http.ResponseWriter, r *http.Request) {
	detail, err := h.accounts.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

type accountPatch struct {
	Blocked *bool `json:"blocked"`
}

// patch muda o bloqueio da conta e devolve a conta como ficou.
func (h *instanceAccountsHandler) patch(w http.ResponseWriter, r *http.Request) {
	var body accountPatch
	if !decodeJSONBody(w, r, &body) {
		return
	}
	if body.Blocked == nil {
		writeError(w, "invalid_request", `missing "blocked": expected {"blocked": true} or {"blocked": false}`, http.StatusBadRequest)
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.accounts.SetBlocked(r.Context(), actor.UserID, id, *body.Blocked); err != nil {
		writeAccountError(w, err)
		return
	}
	h.get(w, r)
}

func (h *instanceAccountsHandler) delete(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	if err := h.accounts.Delete(r.Context(), actor.UserID, chi.URLParam(r, "id")); err != nil {
		writeAccountError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeAccountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrUserNotFound):
		writeError(w, "account_not_found", "account not found", http.StatusNotFound)
	case errors.Is(err, auth.ErrCannotTargetSelf):
		writeError(w, "cannot_target_self", auth.ErrCannotTargetSelf.Error(), http.StatusConflict)
	case errors.Is(err, auth.ErrInvalidAccountQuery):
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
	default:
		writeLoggedInternal(w, "falha nas contas da instancia", "gerir a conta", err)
	}
}

// listSessions lista as sessões da conta sem marcar atual: a do administrador é outra.
func (h *instanceAccountsHandler) listSessions(w http.ResponseWriter, r *http.Request) {
	views, err := h.sessions.ListSessions(r.Context(), chi.URLParam(r, "id"), "")
	if err != nil {
		writeLoggedInternal(w, "falha nas contas da instancia", "listar as sessoes", err)
		return
	}
	writeJSON(w, http.StatusOK, nonNilSessions(views))
}

func (h *instanceAccountsHandler) endSession(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	writeSessionEnd(w, h.sessions.EndSession(r.Context(), actor.UserID, chi.URLParam(r, "id"), chi.URLParam(r, "session_id")))
}

func (h *instanceAccountsHandler) listAuthorizations(w http.ResponseWriter, r *http.Request) {
	views, err := h.apps.ListAuthorizations(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeLoggedInternal(w, "falha nas contas da instancia", "listar os apps autorizados", err)
		return
	}
	writeJSON(w, http.StatusOK, authorizationListFrom(views))
}

func (h *instanceAccountsHandler) revokeAuthorization(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	err := h.apps.RevokeAuthorizationAs(r.Context(), actor.UserID, chi.URLParam(r, "id"), chi.URLParam(r, "authorization_id"))
	writeAuthorizationRevoke(w, err)
}
