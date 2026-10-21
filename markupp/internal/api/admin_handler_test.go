package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

type fakeAdminService struct {
	members    []auth.Member
	invites    []auth.Invite
	issued     auth.IssuedInvite
	issueErr   error
	issuedBy   identity.Actor
	issuedRole string

	changedUser string
	changedRole string
	memberErr   error

	revokedInvite string

	tenantDeletedBy *identity.Actor
	deleteTenantErr error
}

func (f *fakeAdminService) ListMembers(ctx context.Context) ([]auth.Member, error) {
	return f.members, nil
}

func (f *fakeAdminService) ListInvites(ctx context.Context) ([]auth.Invite, error) {
	return f.invites, nil
}

func (f *fakeAdminService) IssueInvite(ctx context.Context, inviter identity.Actor, email, role string) (auth.IssuedInvite, error) {
	f.issuedBy, f.issuedRole = inviter, role
	return f.issued, f.issueErr
}

func adminRequest(svc api.AdminService, role, method, path, body string) *httptest.ResponseRecorder {
	validator := &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}}
	tenants := &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: role}}
	router := api.NewRouter(api.RouterDeps{Admin: svc, Tokens: validator, Tenants: tenants})
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAdminRoutes_PapelSemAdministracao_Retorna403(t *testing.T) {
	for _, papel := range []string{"member", "viewer"} {
		for _, rota := range [][2]string{{http.MethodGet, "/users"}, {http.MethodGet, "/users/invites"}, {http.MethodPost, "/users/invites"}} {
			rec := adminRequest(&fakeAdminService{}, papel, rota[0], rota[1], `{}`)

			assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s %s", papel, rota[0], rota[1])
		}
	}
}

func TestAdminRoutes_Owner_Administra(t *testing.T) {
	rec := adminRequest(&fakeAdminService{}, "owner", http.MethodGet, "/users", "")

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminRoutes_SemToken_Retorna401(t *testing.T) {
	router := api.NewRouter(api.RouterDeps{Admin: &fakeAdminService{}, Tokens: &fakeTokenValidator{}, Tenants: tenantDeTeste})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminRoutes_ListMembers_Admin_DevolveMembrosComPapel(t *testing.T) {
	svc := &fakeAdminService{members: []auth.Member{{UserID: "u1", Email: "a@markupp.dev", Role: "owner"}}}

	rec := adminRequest(svc, "admin", http.MethodGet, "/users", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "a@markupp.dev", resp[0]["email"])
	assert.Equal(t, "owner", resp[0]["role"])
}

func TestAdminRoutes_IssueInvite_DevolveTokenUmaVez(t *testing.T) {
	expira := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	svc := &fakeAdminService{issued: auth.IssuedInvite{
		Invite: auth.Invite{ID: "c1", Email: "nova@markupp.dev", Role: "member", ExpiresAt: expira},
		Token:  "tok-secreto",
	}}

	rec := adminRequest(svc, "admin", http.MethodPost, "/users/invites", `{"email":"nova@markupp.dev","role":"member"}`)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, identity.Actor{UserID: "u1", TenantID: "t1", Role: "admin"}, svc.issuedBy)
	assert.Equal(t, "member", svc.issuedRole)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "tok-secreto", resp["token"])
	assert.Equal(t, "2026-10-21T12:00:00Z", resp["expires_at"])
}

func TestAdminRoutes_IssueInviteComEmail_OmiteOToken(t *testing.T) {
	svc := &fakeAdminService{issued: auth.IssuedInvite{Invite: auth.Invite{ID: "c1", Email: "nova@markupp.dev", Role: "member"}}}

	rec := adminRequest(svc, "admin", http.MethodPost, "/users/invites", `{"email":"nova@markupp.dev","role":"member"}`)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotContains(t, resp, "token", "com SMTP o token sai so no email")
}

func TestAdminRoutes_IssueInvite_ErrosDeDominio(t *testing.T) {
	casos := map[error]int{
		auth.ErrInvalidRole:   http.StatusBadRequest,
		auth.ErrInvalidEmail:  http.StatusBadRequest,
		auth.ErrAlreadyMember: http.StatusConflict,
	}
	for errDominio, status := range casos {
		rec := adminRequest(&fakeAdminService{issueErr: errDominio}, "admin", http.MethodPost, "/users/invites", `{"email":"x","role":"y"}`)

		assert.Equal(t, status, rec.Code, errDominio.Error())
	}
}

func TestAdminRoutes_ListInvites_NaoExpoeToken(t *testing.T) {
	svc := &fakeAdminService{invites: []auth.Invite{{ID: "c1", Email: "nova@markupp.dev", Role: "member"}}}

	rec := adminRequest(svc, "admin", http.MethodGet, "/users/invites", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "nova@markupp.dev")
	assert.NotContains(t, rec.Body.String(), "token")
}
