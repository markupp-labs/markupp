package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// RegistrationService é o que as rotas do cadastro por método consomem (ADR-0059).
type RegistrationService interface {
	Methods(ctx context.Context) ([]instance.RegistrationMethodView, error)
	SetOpen(ctx context.Context, id string, open bool) (instance.RegistrationMethodView, error)
	RegistrationOpen(ctx context.Context, method string) (bool, error)
	RegistrationEnabled() bool
}

type registrationHandler struct {
	svc RegistrationService
}

// accountPolicyResponse é o que o painel precisa, sem conta, para montar a tela de cadastro.
type accountPolicyResponse struct {
	RegistrationEnabled      bool `json:"registration_enabled"`
	PasswordRegistrationOpen bool `json:"password_registration_open"`
}

// mountRegistrationRoutes monta a regra pública do cadastro e, para quem administra a
// instância, a escolha de cada método.
func mountRegistrationRoutes(r chi.Router, svc RegistrationService, accounts AuthService, admins instance.Admins, validator TokenValidator) {
	h := &registrationHandler{svc: svc}
	r.Get("/accounts/policy", h.policy)
	g := r.With(acceptOAuthScope(oauth.ScopeAdmin), RequireAuth(validator), requireInstanceAdmin(accounts, admins))
	g.Get("/instance/registration-methods", h.list)
	g.Patch("/instance/registration-methods/{id}", h.patch)
}

func (h *registrationHandler) policy(w http.ResponseWriter, r *http.Request) {
	open, err := h.svc.RegistrationOpen(r.Context(), "local")
	if err != nil {
		writeLoggedInternal(w, "falha no cadastro por metodo", "ler o cadastro por senha", err)
		return
	}
	writeJSON(w, http.StatusOK, accountPolicyResponse{RegistrationEnabled: h.svc.RegistrationEnabled(), PasswordRegistrationOpen: open})
}

func (h *registrationHandler) list(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.Methods(r.Context())
	if err != nil {
		writeRegistrationMethodError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

func (h *registrationHandler) patch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Open *bool `json:"open"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.Open == nil {
		writeError(w, "invalid_request", `body has no "open", expected {"open": true|false}`, http.StatusBadRequest)
		return
	}
	view, err := h.svc.SetOpen(r.Context(), chi.URLParam(r, "id"), *req.Open)
	if err != nil {
		writeRegistrationMethodError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func writeRegistrationMethodError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, instance.ErrRegistrationLocked):
		writeError(w, "host_locked", err.Error(), http.StatusConflict)
	case errors.Is(err, instance.ErrUnknownRegistrationMethod):
		writeError(w, "not_found", err.Error(), http.StatusNotFound)
	default:
		writeLoggedInternal(w, "falha no cadastro por metodo", "cadastro por metodo da instancia", err)
	}
}
