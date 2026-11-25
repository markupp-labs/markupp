package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
)

type fakeAuditHistory struct {
	entries []auth.AuditEntry
	err     error
	query   auth.AuditQuery
}

func (f *fakeAuditHistory) List(_ context.Context, q auth.AuditQuery) ([]auth.AuditEntry, error) {
	f.query = q
	return f.entries, f.err
}

func auditEventsRequest(svc *fakeAuditHistory, query string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Auth: &fakeAuthService{accountUser: administradorDaInstancia}, Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}},
		InstanceAdmins: instance.Admins{emailDoAdministrador}, AuditHistory: svc,
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/instance/audit-events"+query, nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAuditEvents_Listar_PassaOsFiltrosEDevolveOHistorico(t *testing.T) {
	quando := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	svc := &fakeAuditHistory{entries: []auth.AuditEntry{{ID: "e1", ActorID: "u2", ActorEmail: "bia@exemplo.org",
		Action: "auth.login", Target: "bia@exemplo.org", Result: "failure", IP: "198.51.100.9", UserAgent: "curl/8", OccurredAt: quando}}}

	rec := auditEventsRequest(svc, "?account=u2&action=auth.login&result=failure&ip=198.51.100.9"+
		"&since=2026-11-01T00:00:00Z&until=2026-11-30T00:00:00Z&offset=5&limit=10")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"id":"e1","actor_id":"u2","actor_email":"bia@exemplo.org","action":"auth.login",
		"target":"bia@exemplo.org","result":"failure","ip":"198.51.100.9","user_agent":"curl/8",
		"occurred_at":"2026-11-25T12:00:00Z"}]`, rec.Body.String())
	desde, ate := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, auth.AuditQuery{Account: "u2", Action: "auth.login", Result: "failure", IP: "198.51.100.9",
		Since: &desde, Until: &ate, Offset: 5, Limit: 10}, svc.query)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestAuditEvents_Vazio_Array(t *testing.T) {
	rec := auditEventsRequest(&fakeAuditHistory{}, "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestAuditEvents_FiltroInvalido_400(t *testing.T) {
	for _, query := range []string{"?since=ontem", "?until=2026-13-01T00:00:00Z", "?limit=500"} {
		rec := auditEventsRequest(&fakeAuditHistory{}, query)

		assert.Equal(t, http.StatusBadRequest, rec.Code, query)
		assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`, query)
	}
}

func TestAuditEvents_ServicoRecusaOFiltro_400(t *testing.T) {
	rec := auditEventsRequest(&fakeAuditHistory{err: fmt.Errorf("%w: result %q", auth.ErrInvalidAuditQuery, "x")}, "?result=x")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
