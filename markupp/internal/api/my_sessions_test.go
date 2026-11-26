package api_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

type fakeSessionService struct {
	sessions []auth.SessionView
	err      error

	gotOwner, gotCurrent, gotActor, gotSessionID string
}

func (f *fakeSessionService) ListSessions(_ context.Context, userID, currentSessionID string) ([]auth.SessionView, error) {
	f.gotOwner, f.gotCurrent = userID, currentSessionID
	return f.sessions, f.err
}

func (f *fakeSessionService) EndSession(_ context.Context, actorID, userID, sessionID string) error {
	f.gotActor, f.gotOwner, f.gotSessionID = actorID, userID, sessionID
	return f.err
}

func mySessionsRequest(svc *fakeSessionService, claims auth.Claims, method, path string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{Tokens: &fakeTokenValidator{claims: claims}, Probe: &fakeProbe{}, Sessions: svc})
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

var loginDoPainel = auth.Claims{Subject: "u1", SessionID: "s-atual"}

func TestMySessions_Listar_DevolveAsSessoesComAAtualMarcada(t *testing.T) {
	quando := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	svc := &fakeSessionService{sessions: []auth.SessionView{{
		ID: "s-atual", IP: "203.0.113.7", UserAgent: "Firefox/140", CreatedAt: quando, LastUsedAt: quando, Current: true,
	}}}

	rec := mySessionsRequest(svc, loginDoPainel, http.MethodGet, "/v1/me/sessions")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[{"id":"s-atual","ip":"203.0.113.7","user_agent":"Firefox/140",
		"created_at":"2026-11-25T12:00:00Z","last_used_at":"2026-11-25T12:00:00Z","current":true}]`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "u1", svc.gotOwner)
	assert.Equal(t, "s-atual", svc.gotCurrent)
}

func TestMySessions_ListarVazio_DevolveListaVazia(t *testing.T) {
	rec := mySessionsRequest(&fakeSessionService{}, loginDoPainel, http.MethodGet, "/v1/me/sessions")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestMySessions_Encerrar_Retorna204(t *testing.T) {
	svc := &fakeSessionService{}

	rec := mySessionsRequest(svc, loginDoPainel, http.MethodDelete, "/v1/me/sessions/s-outra")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "u1", svc.gotActor)
	assert.Equal(t, "u1", svc.gotOwner)
	assert.Equal(t, "s-outra", svc.gotSessionID)
}

func TestMySessions_EncerrarInexistente_Retorna404(t *testing.T) {
	svc := &fakeSessionService{err: fmt.Errorf("encerrar: %w", auth.ErrSessionNotFound)}

	rec := mySessionsRequest(svc, loginDoPainel, http.MethodDelete, "/v1/me/sessions/s-outra")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"session_not_found"`)
}

func TestMySessions_ErroInterno_Retorna500(t *testing.T) {
	rec := mySessionsRequest(&fakeSessionService{err: errors.New("banco fora")}, loginDoPainel, http.MethodGet, "/v1/me/sessions")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"internal","message":"internal error"}`, rec.Body.String())
}

func TestMySessions_TokenOAuthComEscopoAdmin_Lista(t *testing.T) {
	svc := &fakeSessionService{}

	rec := mySessionsRequest(svc, auth.Claims{Subject: "u1", Scope: "administracao"}, http.MethodGet, "/v1/me/sessions")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, svc.gotCurrent, "token OAuth não vem de sessão de login")
}

func TestMySessions_TokenOAuthSemEscopoAdmin_Retorna403(t *testing.T) {
	svc := &fakeSessionService{}

	rec := mySessionsRequest(svc, auth.Claims{Subject: "u1", Scope: "notas:leitura"}, http.MethodDelete, "/v1/me/sessions/s-outra")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, svc.gotSessionID)
}
