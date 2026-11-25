package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// RetentionService é o prazo de guarda do histórico e do IP (ADR-0061).
type RetentionService interface {
	Settings(ctx context.Context) (instance.RetentionView, error)
	Update(ctx context.Context, actorID string, days int) (instance.RetentionView, error)
}

type retentionUpdate struct {
	Days *int `json:"days"`
}

// mountRetentionRoutes monta /instance/retention com o escopo de administração e só para
// quem administra a instância.
func mountRetentionRoutes(r chi.Router, svc RetentionService, accounts AuthService, admins instance.Admins, validator TokenValidator) {
	g := r.With(acceptOAuthScope(oauth.ScopeAdmin), RequireAuth(validator), requireInstanceAdmin(accounts, admins), noStore)
	g.Get("/instance/retention", func(w http.ResponseWriter, r *http.Request) {
		view, err := svc.Settings(r.Context())
		writeRetention(w, view, err)
	})
	g.Put("/instance/retention", func(w http.ResponseWriter, r *http.Request) { updateRetention(w, r, svc) })
}

func updateRetention(w http.ResponseWriter, r *http.Request, svc RetentionService) {
	var body retentionUpdate
	if !decodeJSONBody(w, r, &body) {
		return
	}
	if body.Days == nil {
		writeError(w, "invalid_request", `missing "days": expected {"days": 365}`, http.StatusBadRequest)
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	view, err := svc.Update(r.Context(), actor.UserID, *body.Days)
	writeRetention(w, view, err)
}

func writeRetention(w http.ResponseWriter, view instance.RetentionView, err error) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, view)
	case errors.Is(err, instance.ErrHostLocked):
		writeError(w, "host_locked", "the retention is set by the host configuration", http.StatusConflict)
	case errors.Is(err, instance.ErrInvalidRetention):
		writeError(w, "invalid_retention", err.Error(), http.StatusBadRequest)
	default:
		writeLoggedInternal(w, "falha na administracao da instancia", "prazo de guarda", err)
	}
}
