package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// AdminService é o que as rotas de administração consomem.
type AdminService interface {
	ListMembers(ctx context.Context) ([]auth.Member, error)
	ListInvites(ctx context.Context) ([]auth.Invite, error)
	IssueInvite(ctx context.Context, inviter identity.Actor, email, role string) (auth.IssuedInvite, error)
	ChangeRole(ctx context.Context, actor identity.Actor, userID, role string) error
	RemoveMember(ctx context.Context, actor identity.Actor, userID string) error
	RevokeInvite(ctx context.Context, actor identity.Actor, inviteID string) error
}

type adminHandler struct {
	svc AdminService
}

type inviteResponse struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	InvitedBy  string     `json:"invited_by,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type issuedInviteResponse struct {
	inviteResponse
	// Token sai vazio quando o convite vai por email, e o campo some da resposta.
	Token string `json:"token,omitempty"`
}

// mountAdminRoutes monta as rotas que só owner e admin do tenant alcançam. O papel vem
// do vínculo no banco, nunca do cliente.
func mountAdminRoutes(r chi.Router, svc AdminService, validator TokenValidator, tenants TenantResolver) {
	h := &adminHandler{svc: svc}
	r.Group(func(g chi.Router) {
		g.Use(RequireAuth(validator), requireOAuthScope(oauth.ScopeAdmin), RequireTenant(tenants), requireAdmin)
		g.Get("/users", h.listMembers)
		g.Get("/users/invites", h.listInvites)
		g.Post("/users/invites", h.issueInvite)
		g.Patch("/users/{id}", h.changeRole)
		g.Delete("/users/{id}", h.removeMember)
		g.Delete("/users/invites/{id}", h.revokeInvite)
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

type memberResponse struct {
	ID       string    `json:"id"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

func (h *adminHandler) listMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.svc.ListMembers(r.Context())
	if err != nil {
		writeAdminInternal(w, "listar membros", err)
		return
	}
	out := make([]memberResponse, 0, len(members))
	for _, m := range members {
		out = append(out, memberResponse{ID: m.UserID, Email: m.Email, Role: m.Role, JoinedAt: m.JoinedAt})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *adminHandler) listInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := h.svc.ListInvites(r.Context())
	if err != nil {
		writeAdminInternal(w, "listar convites", err)
		return
	}
	out := make([]inviteResponse, 0, len(invites))
	for _, inv := range invites {
		out = append(out, toInviteResponse(inv))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *adminHandler) issueInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	issued, err := h.svc.IssueInvite(r.Context(), actor, req.Email, req.Role)
	if err != nil {
		writeAdminError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, issuedInviteResponse{inviteResponse: toInviteResponse(issued.Invite), Token: issued.Token})
}

func (h *adminHandler) changeRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Role string `json:"role"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	h.answerMembership(w, h.svc.ChangeRole(r.Context(), actor, chi.URLParam(r, "id"), req.Role))
}

func (h *adminHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	h.answerMembership(w, h.svc.RemoveMember(r.Context(), actor, chi.URLParam(r, "id")))
}

func (h *adminHandler) revokeInvite(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	err := h.svc.RevokeInvite(r.Context(), actor, chi.URLParam(r, "id"))
	if errors.Is(err, auth.ErrInviteInvalid) {
		writeError(w, "invite_not_found", "convite inexistente ou ja respondido", http.StatusNotFound)
		return
	}
	h.answerMembership(w, err)
}

func (h *adminHandler) answerMembership(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, auth.ErrMembershipNotFound):
		writeError(w, "member_not_found", "usuario nao e membro deste tenant", http.StatusNotFound)
	case errors.Is(err, auth.ErrLastOwner):
		writeError(w, "last_owner", err.Error(), http.StatusConflict)
	default:
		writeAdminError(w, err)
	}
}

func writeAdminError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidRole), errors.Is(err, auth.ErrInvalidEmail):
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
	case errors.Is(err, auth.ErrAlreadyMember):
		writeError(w, "already_member", err.Error(), http.StatusConflict)
	case errors.Is(err, auth.ErrRoleAboveOwn):
		writeError(w, "role_above_own", err.Error(), http.StatusForbidden)
	default:
		writeAdminInternal(w, "administrar o tenant", err)
	}
}

func toInviteResponse(inv auth.Invite) inviteResponse {
	return inviteResponse{
		ID: inv.ID, Email: inv.Email, Role: inv.Role, InvitedBy: inv.InvitedBy,
		ExpiresAt: inv.ExpiresAt, AcceptedAt: inv.AcceptedAt, CreatedAt: inv.CreatedAt,
	}
}

func writeAdminInternal(w http.ResponseWriter, operation string, err error) {
	writeLoggedInternal(w, "falha na administracao", operation, err)
}

// writeLoggedInternal registra a falha com a mensagem da área e responde 500 sem expor
// a causa ao cliente.
func writeLoggedInternal(w http.ResponseWriter, message, operation string, err error) {
	slog.Error(message, "operacao", operation, "error", err)
	writeError(w, "internal", "erro interno", http.StatusInternalServerError)
}
