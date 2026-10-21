package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

type pendingInviteResponse struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	TenantName string    `json:"tenant_name"`
	Role       string    `json:"role"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func (h *authHandler) myInvites(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	invites, err := h.svc.ListMyInvites(r.Context(), actor.UserID)
	if err != nil {
		writeMyInvitesInternal(w, "listar convites da conta", err)
		return
	}
	out := make([]pendingInviteResponse, 0, len(invites))
	for _, inv := range invites {
		out = append(out, pendingInviteResponse(inv))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *authHandler) acceptMyInvite(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	m, err := h.svc.AcceptMyInvite(r.Context(), actor.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeAnswerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tenantResponse{ID: m.TenantID, Name: m.TenantName, Role: m.Role})
}

func (h *authHandler) declineMyInvite(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	if err := h.svc.DeclineMyInvite(r.Context(), actor.UserID, chi.URLParam(r, "id")); err != nil {
		writeAnswerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeAnswerError não distingue convite inexistente de convite de outra conta.
func writeAnswerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInviteInvalid):
		writeError(w, "invite_not_found", "convite inexistente, vencido ou ja respondido", http.StatusNotFound)
	case errors.Is(err, auth.ErrAlreadyMember):
		writeError(w, "already_member", "a conta ja e membro desse tenant", http.StatusConflict)
	default:
		writeMyInvitesInternal(w, "responder convite", err)
	}
}

func writeMyInvitesInternal(w http.ResponseWriter, operation string, err error) {
	writeLoggedInternal(w, "falha nos convites da conta", operation, err)
}
