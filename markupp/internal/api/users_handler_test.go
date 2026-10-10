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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/users"
)

type fakeUsersService struct {
	bootstrapUser users.User
	bootstrapErr  error
	inviteResult  users.UserInvite
	inviteErr     error
	listResult    []users.User
	listErr       error
	invitesResult []users.UserInvite
	invitesErr    error
}

func (f *fakeUsersService) BootstrapFirstAdmin(ctx context.Context, tenantID, email string) (users.User, error) {
	return f.bootstrapUser, f.bootstrapErr
}

func (f *fakeUsersService) InviteUser(ctx context.Context, tenantID, actorRole, actorID, targetEmail, role string) (users.UserInvite, error) {
	return f.inviteResult, f.inviteErr
}

func (f *fakeUsersService) ListUsers(ctx context.Context, tenantID string) ([]users.User, error) {
	return f.listResult, f.listErr
}

func (f *fakeUsersService) ListInvites(ctx context.Context, tenantID string) ([]users.UserInvite, error) {
	return f.invitesResult, f.invitesErr
}

func setupUsersRouter(svc api.UsersService) http.Handler {
	return api.NewRouterWithUsers(&fakeService{}, svc, &fakeProbe{}, nil)
}

func TestUsersHandler_BootstrapAdmin_Sucesso(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	svc := &fakeUsersService{
		bootstrapUser: users.User{
			ID:        "admin-id",
			TenantID:  "tenant-1",
			Email:     "admin@markupp.dev",
			Role:      "admin",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	router := setupUsersRouter(svc)

	body := `{"email":"admin@markupp.dev"}`
	req := httptest.NewRequest(http.MethodPost, "/users/bootstrap-admin", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp users.User
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "admin@markupp.dev", resp.Email)
	assert.Equal(t, "admin", resp.Role)
}

func TestUsersHandler_BootstrapAdmin_JSONInvalidoRetorna400(t *testing.T) {
	router := setupUsersRouter(&fakeUsersService{})

	req := httptest.NewRequest(http.MethodPost, "/users/bootstrap-admin", bytes.NewBufferString("{invalido"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
}

func TestUsersHandler_Invite_AdminSucesso(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	svc := &fakeUsersService{
		inviteResult: users.UserInvite{
			ID:        "inv-id",
			TenantID:  "tenant-1",
			Email:     "membro@markupp.dev",
			Role:      "member",
			InvitedBy: "admin-id",
			Status:    "pending",
			CreatedAt: now,
		},
	}
	router := setupUsersRouter(svc)

	body := `{"email":"membro@markupp.dev","role":"member"}`
	req := httptest.NewRequest(http.MethodPost, "/users/invite", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	req.Header.Set("X-User-ID", "admin-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp users.UserInvite
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "membro@markupp.dev", resp.Email)
}

func TestUsersHandler_Invite_NaoAdminRetorna403(t *testing.T) {
	svc := &fakeUsersService{inviteErr: users.ErrOnlyAdminCanInvite}
	router := setupUsersRouter(svc)

	body := `{"email":"membro@markupp.dev","role":"member"}`
	req := httptest.NewRequest(http.MethodPost, "/users/invite", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "member")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUsersHandler_ListUsers_Sucesso(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	svc := &fakeUsersService{
		listResult: []users.User{
			{ID: "u1", TenantID: "default", Email: "admin@markupp.dev", Role: "admin", CreatedAt: now, UpdatedAt: now},
		},
	}
	router := setupUsersRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("X-User-Role", "admin")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []users.User
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
}
