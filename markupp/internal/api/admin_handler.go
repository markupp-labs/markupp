package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

// AdminService é o que as rotas de administração consomem.
type AdminService interface {
	ListUsers(ctx context.Context) ([]auth.User, error)
	ListInvites(ctx context.Context) ([]auth.Invite, error)
	IssueInvite(ctx context.Context, inviter identity.Actor, email, role string) (auth.IssuedInvite, error)
}

type adminHandler struct {
	svc AdminService
}

type inviteResponse struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	InvitedBy  string     `json:"invited_by"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type issuedInviteResponse struct {
	inviteResponse
	Token string `json:"token"`
}

// MountAdminRoutes monta as rotas que só o administrador do tenant alcança. O papel
// vem do JWT validado, nunca de cabeçalho enviado pelo cliente.
func MountAdminRoutes(r chi.Router, svc AdminService, validator TokenValidator) {
	h := &adminHandler{svc: svc}
	r.Group(func(g chi.Router) {
		g.Use(RequireAuth(validator), requireAdmin)
		g.Get("/users", h.listUsers)
		g.Get("/users/invites", h.listInvites)
		g.Post("/users/invites", h.issueInvite)
	})
}

func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := identity.ActorFrom(r.Context())
		if !ok || !actor.IsAdmin() {
			writeError(w, "forbidden", "acesso restrito a administradores", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *adminHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers(r.Context())
	if err != nil {
		writeInternal(w, "listar usuarios", err)
		return
	}
	out := make([]userResponse, 0, len(users))
	for _, u := range users {
		out = append(out, userResponse{ID: u.ID, TenantID: u.TenantID, Email: u.Email, Role: u.Role})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *adminHandler) listInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := h.svc.ListInvites(r.Context())
	if err != nil {
		writeInternal(w, "listar convites", err)
		return
	}
	out := make([]inviteResponse, 0, len(invites))
	for _, inv := range invites {
		out = append(out, toInviteResponse(inv))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *adminHandler) issueInvite(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "JSON invalido", http.StatusBadRequest)
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	issued, err := h.svc.IssueInvite(r.Context(), actor, req.Email, req.Role)
	if err != nil {
		writeInviteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, issuedInviteResponse{inviteResponse: toInviteResponse(issued.Invite), Token: issued.Token})
}

func writeInviteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidRole), errors.Is(err, auth.ErrInvalidEmail):
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
	case errors.Is(err, auth.ErrAlreadyMember):
		writeError(w, "already_member", err.Error(), http.StatusConflict)
	default:
		writeInternal(w, "emitir convite", err)
	}
}

func toInviteResponse(inv auth.Invite) inviteResponse {
	return inviteResponse{
		ID: inv.ID, Email: inv.Email, Role: inv.Role, InvitedBy: inv.InvitedBy,
		ExpiresAt: inv.ExpiresAt, AcceptedAt: inv.AcceptedAt, CreatedAt: inv.CreatedAt,
	}
}

func writeInternal(w http.ResponseWriter, operation string, err error) {
	slog.Error("falha na administracao", "operacao", operation, "error", err)
	writeError(w, "internal", "erro interno", http.StatusInternalServerError)
}
