package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/tenant"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/users"
)

// UsersService define os métodos do domínio de usuários expostos via HTTP.
type UsersService interface {
	BootstrapFirstAdmin(ctx context.Context, tenantID, email string) (users.User, error)
	InviteUser(ctx context.Context, tenantID, actorRole, actorID, targetEmail, role string) (users.UserInvite, error)
	ListUsers(ctx context.Context, tenantID string) ([]users.User, error)
	ListInvites(ctx context.Context, tenantID string) ([]users.UserInvite, error)
}

type usersHandler struct {
	svc UsersService
}

type bootstrapRequest struct {
	Email string `json:"email"`
}

type inviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (h *usersHandler) bootstrapAdmin(w http.ResponseWriter, r *http.Request) {
	var req bootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "JSON invalido", http.StatusBadRequest)
		return
	}
	tenantID := tenant.IDFromContext(r.Context(), "default")
	u, err := h.svc.BootstrapFirstAdmin(r.Context(), tenantID, req.Email)
	if err != nil {
		h.handleUserError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (h *usersHandler) invite(w http.ResponseWriter, r *http.Request) {
	var req inviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "JSON invalido", http.StatusBadRequest)
		return
	}
	tc, _ := tenant.FromContext(r.Context())
	role := req.Role
	if role == "" {
		role = "member"
	}
	inv, err := h.svc.InviteUser(r.Context(), tc.TenantID, tc.Role, tc.UserID, req.Email, role)
	if err != nil {
		h.handleUserError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, inv)
}

func (h *usersHandler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.IDFromContext(r.Context(), "default")
	list, err := h.svc.ListUsers(r.Context(), tenantID)
	if err != nil {
		writeError(w, "internal", "erro ao listar usuarios", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *usersHandler) listInvites(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.IDFromContext(r.Context(), "default")
	list, err := h.svc.ListInvites(r.Context(), tenantID)
	if err != nil {
		writeError(w, "internal", "erro ao listar convites", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *usersHandler) handleUserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, users.ErrOnlyAdminCanInvite):
		writeError(w, "forbidden", err.Error(), http.StatusForbidden)
	case errors.Is(err, users.ErrInvalidEmail):
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
	case errors.Is(err, users.ErrUserAlreadyExists), errors.Is(err, users.ErrInviteAlreadyExists):
		writeError(w, "conflict", err.Error(), http.StatusConflict)
	default:
		writeError(w, "internal", "erro interno", http.StatusInternalServerError)
	}
}
