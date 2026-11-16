package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// InstanceEmbeddingService é o que as rotas do embedding da instância consomem.
type InstanceEmbeddingService interface {
	Settings(ctx context.Context) (instance.EmbeddingView, error)
	Update(ctx context.Context, update instance.EmbeddingUpdate) (instance.EmbeddingView, error)
}

type instanceHandler struct {
	svc InstanceEmbeddingService
}

// instanceEmbeddingRequest é a escolha pedida. api_key ausente ou null mantém a chave
// guardada, e vazia a apaga.
type instanceEmbeddingRequest struct {
	Provider string  `json:"provider"`
	URL      string  `json:"url"`
	Model    string  `json:"model"`
	APIKey   *string `json:"api_key"`
}

// mountInstanceRoutes monta as rotas da instância, só sob /v1, com o escopo de
// administração e só para quem administra a instância.
func mountInstanceRoutes(r chi.Router, svc InstanceEmbeddingService, accounts AuthService, admins instance.Admins, validator TokenValidator) {
	h := &instanceHandler{svc: svc}
	g := r.With(acceptOAuthScope(oauth.ScopeAdmin), RequireAuth(validator), requireInstanceAdmin(accounts, admins))
	g.Get("/instance/embedding", h.get)
	g.Put("/instance/embedding", h.put)
}

// requireInstanceAdmin deixa passar só a conta de email verificado na lista da instância.
func requireInstanceAdmin(accounts AuthService, admins instance.Admins) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, _ := identity.ActorFrom(r.Context())
			user, err := accounts.GetAccount(r.Context(), actor.UserID)
			if err != nil {
				writeLoggedInternal(w, "falha na administracao da instancia", "ler a conta", err)
				return
			}
			if !admins.Includes(user.Email, user.EmailVerified) {
				writeError(w, "instance_admin_required", fmt.Sprintf("account %q is not an instance admin, expected a verified email listed by whoever hosts this instance", user.Email), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (h *instanceHandler) get(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.Settings(r.Context())
	if err != nil {
		writeInstanceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *instanceHandler) put(w http.ResponseWriter, r *http.Request) {
	var req instanceEmbeddingRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	update := instance.EmbeddingUpdate{Provider: req.Provider, URL: req.URL, Model: req.Model, APIKey: req.APIKey}
	view, err := h.svc.Update(r.Context(), update)
	if err != nil {
		writeInstanceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func writeInstanceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, instance.ErrInvalidEmbedding):
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
	case errors.Is(err, instance.ErrHostLocked):
		writeError(w, "host_locked", err.Error(), http.StatusConflict)
	default:
		writeLoggedInternal(w, "falha na administracao da instancia", "embedding da instancia", err)
	}
}
