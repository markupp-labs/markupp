package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// InstanceLoginProviderService é o que as rotas dos provedores de login da instância
// consomem (ADR-0060).
type InstanceLoginProviderService interface {
	List(ctx context.Context) (instance.LoginProvidersView, error)
	Create(ctx context.Context, input instance.LoginProviderInput) (instance.LoginProviderView, error)
	Update(ctx context.Context, input instance.LoginProviderInput) (instance.LoginProviderView, error)
	Delete(ctx context.Context, id string) (int, error)
}

type loginProvidersAdminHandler struct {
	svc InstanceLoginProviderService
}

// loginProviderRequest é o provedor pedido. client_secret ausente, null ou vazio mantém o
// guardado; na criação ele é obrigatório.
type loginProviderRequest struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Issuer       string  `json:"issuer"`
	ClientID     string  `json:"client_id"`
	ClientSecret *string `json:"client_secret"`
	Enabled      bool    `json:"enabled"`
}

func (req loginProviderRequest) input(id string) instance.LoginProviderInput {
	return instance.LoginProviderInput{ID: id, Name: req.Name, Issuer: req.Issuer, ClientID: req.ClientID, ClientSecret: req.ClientSecret, Enabled: req.Enabled}
}

// mountLoginProvidersAdminRoutes monta a lista dos provedores de login, só sob /v1, com o
// escopo de administração e só para quem administra a instância.
func mountLoginProvidersAdminRoutes(r chi.Router, svc InstanceLoginProviderService, accounts AuthService, admins instance.Admins, validator TokenValidator) {
	h := &loginProvidersAdminHandler{svc: svc}
	g := r.With(acceptOAuthScope(oauth.ScopeAdmin), RequireAuth(validator), requireInstanceAdmin(accounts, admins))
	g.Get("/instance/login-providers", h.list)
	g.Post("/instance/login-providers", h.create)
	g.Put("/instance/login-providers/{id}", h.update)
	g.Delete("/instance/login-providers/{id}", h.delete)
}

func (h *loginProvidersAdminHandler) list(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.List(r.Context())
	if err != nil {
		writeLoginProviderAdminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *loginProvidersAdminHandler) create(w http.ResponseWriter, r *http.Request) {
	var req loginProviderRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	view, err := h.svc.Create(r.Context(), req.input(req.ID))
	if err != nil {
		writeLoginProviderAdminError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

// update usa o id da rota: o id não muda depois de criado.
func (h *loginProvidersAdminHandler) update(w http.ResponseWriter, r *http.Request) {
	var req loginProviderRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	view, err := h.svc.Update(r.Context(), req.input(chi.URLParam(r, "id")))
	if err != nil {
		writeLoginProviderAdminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *loginProvidersAdminHandler) delete(w http.ResponseWriter, r *http.Request) {
	removed, err := h.svc.Delete(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeLoginProviderAdminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"removed_links": removed})
}

func writeLoginProviderAdminError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, instance.ErrInvalidLoginProvider):
		writeError(w, "invalid_login_provider", err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, instance.ErrLoginProvidersLocked):
		writeError(w, "host_locked", err.Error(), http.StatusConflict)
	case errors.Is(err, instance.ErrLoginProviderIssuerLocked):
		writeError(w, "login_provider_issuer_locked", err.Error(), http.StatusConflict)
	case errors.Is(err, instance.ErrLoginProviderExists):
		writeError(w, "login_provider_exists", err.Error(), http.StatusConflict)
	case errors.Is(err, instance.ErrLoginProviderNotFound):
		writeError(w, "login_provider_not_found", err.Error(), http.StatusNotFound)
	default:
		writeLoggedInternal(w, "falha na administracao da instancia", "provedores de login da instancia", err)
	}
}
