package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func myInviteRequest(svc *fakeAuthService, method, path string) *httptest.ResponseRecorder {
	router := setupAuthRouter(svc, &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}})
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestMyInvites_Lista_DevolvePendentesDaConta(t *testing.T) {
	expira := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	svc := &fakeAuthService{pending: []auth.PendingInvite{{ID: "c1", TenantID: "t1", TenantName: "Acme", Role: "viewer", ExpiresAt: expira}}}

	rec := myInviteRequest(svc, http.MethodGet, "/me/invites")

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, []map[string]any{{"id": "c1", "tenant_id": "t1", "tenant_name": "Acme", "role": "viewer", "expires_at": "2026-10-22T12:00:00Z"}}, resp)
}

func TestMyInvites_Aceitar_DevolveOTenant(t *testing.T) {
	svc := &fakeAuthService{acceptedMembership: auth.Membership{TenantID: "t1", TenantName: "Acme", Role: "viewer"}}

	rec := myInviteRequest(svc, http.MethodPost, "/me/invites/c1/accept")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "c1", svc.answeredInvite)
	assert.Contains(t, rec.Body.String(), `"role":"viewer"`)
}

func TestMyInvites_Recusar_Retorna204(t *testing.T) {
	svc := &fakeAuthService{}

	rec := myInviteRequest(svc, http.MethodPost, "/me/invites/c1/decline")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "c1", svc.answeredInvite)
}

func TestMyInvites_ErrosDoAceite(t *testing.T) {
	casos := map[error]int{auth.ErrInviteInvalid: http.StatusNotFound, auth.ErrAlreadyMember: http.StatusConflict}
	for errAceite, status := range casos {
		rec := myInviteRequest(&fakeAuthService{answerErr: errAceite}, http.MethodPost, "/me/invites/c1/accept")

		assert.Equal(t, status, rec.Code, errAceite.Error())
	}
}
