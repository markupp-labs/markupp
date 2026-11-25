package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// SessionService lista e encerra as sessões de login de uma conta (ADR-0061).
type SessionService interface {
	ListSessions(ctx context.Context, userID, currentSessionID string) ([]auth.SessionView, error)
	EndSession(ctx context.Context, actorID, userID, sessionID string) error
}

type mySessionsHandler struct {
	svc SessionService
}

// mountMySessionRoutes monta as sessões da conta. Sem tenant, porque a sessão é da conta;
// o token OAuth só chega com o escopo administracao.
func mountMySessionRoutes(r chi.Router, svc SessionService, tokenValidator TokenValidator) {
	h := &mySessionsHandler{svc: svc}
	me := r.With(acceptOAuthScope(oauth.ScopeAdmin), RequireAuth(tokenValidator))
	me.Get("/me/sessions", h.list)
	me.Delete("/me/sessions/{id}", h.end)
}

func (h *mySessionsHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, _ := identity.ActorFrom(r.Context())
	views, err := h.svc.ListSessions(r.Context(), actor.UserID, actor.SessionID)
	if err != nil {
		writeLoggedInternal(w, "falha nas sessoes da conta", "listar as sessoes", err)
		return
	}
	writeJSON(w, http.StatusOK, nonNilSessions(views))
}

// nonNilSessions troca a lista nula por vazia, para o JSON sempre trazer array.
func nonNilSessions(views []auth.SessionView) []auth.SessionView {
	if views == nil {
		return []auth.SessionView{}
	}
	return views
}

func (h *mySessionsHandler) end(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, _ := identity.ActorFrom(r.Context())
	writeSessionEnd(w, h.svc.EndSession(r.Context(), actor.UserID, actor.UserID, chi.URLParam(r, "id")))
}

// writeSessionEnd responde ao encerramento sem distinguir sessão inexistente, já
// encerrada ou de outra conta.
func writeSessionEnd(w http.ResponseWriter, err error) {
	if errors.Is(err, auth.ErrSessionNotFound) {
		writeError(w, "session_not_found", "session not found or already ended", http.StatusNotFound)
		return
	}
	if err != nil {
		writeLoggedInternal(w, "falha nas sessoes da conta", "encerrar a sessao", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
