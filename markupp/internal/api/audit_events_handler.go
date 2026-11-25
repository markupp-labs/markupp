package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

const maxAuditPageSize = 200

// AuditHistoryService lê o histórico de acessos e mudanças sensíveis (ADR-0061).
type AuditHistoryService interface {
	List(ctx context.Context, q auth.AuditQuery) ([]auth.AuditEntry, error)
}

// mountAuditEventRoutes monta /instance/audit-events com o escopo de administração e só
// para quem administra a instância.
func mountAuditEventRoutes(r chi.Router, svc AuditHistoryService, accounts AuthService, admins instance.Admins, validator TokenValidator) {
	g := r.With(acceptOAuthScope(oauth.ScopeAdmin), RequireAuth(validator), requireInstanceAdmin(accounts, admins), noStore)
	g.Get("/instance/audit-events", func(w http.ResponseWriter, r *http.Request) { listAuditEvents(w, r, svc) })
}

func listAuditEvents(w http.ResponseWriter, r *http.Request, svc AuditHistoryService) {
	q, err := auditQueryFrom(r)
	if err != nil {
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
		return
	}
	entries, err := svc.List(r.Context(), q)
	if errors.Is(err, auth.ErrInvalidAuditQuery) {
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		writeLoggedInternal(w, "falha no historico da instancia", "listar o historico", err)
		return
	}
	if entries == nil {
		entries = []auth.AuditEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func auditQueryFrom(r *http.Request) (auth.AuditQuery, error) {
	qr := &queryReader{values: r.URL.Query()}
	q := auth.AuditQuery{
		Account: qr.values.Get("account"), Action: qr.values.Get("action"),
		Result: qr.values.Get("result"), IP: qr.values.Get("ip"),
		Offset: qr.intBetween("offset", 0, 0, maxOffset),
		Limit:  qr.intBetween("limit", 0, 1, maxAuditPageSize),
	}
	if qr.err != nil {
		return auth.AuditQuery{}, qr.err
	}
	var err error
	if q.Since, err = timestampParam(r, "since"); err != nil {
		return auth.AuditQuery{}, err
	}
	q.Until, err = timestampParam(r, "until")
	return q, err
}

// timestampParam lê name em RFC 3339, nil quando ausente.
func timestampParam(r *http.Request, name string) (*time.Time, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s %q: expected an RFC 3339 timestamp such as 2026-11-25T12:00:00Z", errInvalidQuery, name, raw)
	}
	return &t, nil
}
