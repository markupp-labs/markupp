package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

func setupAuthIntegrationServer(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	pool := storagetest.EmptyPool(t)
	require.NoError(t, storage.Migrate(context.Background(), pool))

	notesRepo := storage.NewPostgresNotesRepository(pool)
	notesSvc := notes.NewService(notesRepo, integrationMaxNoteSize)

	authRepo := storage.NewPostgresAuthRepository(pool)
	codec := auth.NewTokenCodec("segredo-para-teste-de-integracao-de-auth", 15*time.Minute, time.Now)
	hasher := auth.NewBcryptHasher()
	authCfg := auth.ServiceConfig{
		DefaultTenantID:         "default",
		RegistrationEnabled:     false,
		LocalEnabled:            true,
		RefreshExpirationPeriod: 7 * 24 * time.Hour,
	}
	authSvc := auth.NewService(authRepo, codec, hasher, authCfg, time.Now)
	router := api.NewRouterWithAuth(notesSvc, authSvc, codec, pool, nil)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server, pool
}

func postJSON(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func getWithAuth(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestIntegration_Auth_FluxoCompleto(t *testing.T) {
	server, db := setupAuthIntegrationServer(t)

	loginResp := postJSON(t, server.URL+"/auth/login", `{"email":"admin@markupp.dev","password":"senhaForte123"}`)
	defer func() { _ = loginResp.Body.Close() }()
	require.Equal(t, http.StatusOK, loginResp.StatusCode)

	tokens := decodeJSON(t, loginResp.Body)
	accessToken, _ := tokens["access_token"].(string)
	refreshToken, _ := tokens["refresh_token"].(string)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)

	meResp := getWithAuth(t, server.URL+"/auth/me", accessToken)
	defer func() { _ = meResp.Body.Close() }()
	require.Equal(t, http.StatusOK, meResp.StatusCode)

	meBody := decodeJSON(t, meResp.Body)
	assert.Equal(t, "admin@markupp.dev", meBody["email"])
	assert.Equal(t, "admin", meBody["role"])
	assert.Equal(t, "default", meBody["tenant_id"])

	var userCount, vaultCount, auditCount int
	require.NoError(t, db.QueryRow(context.Background(), "SELECT COUNT(*) FROM users").Scan(&userCount))
	require.NoError(t, db.QueryRow(context.Background(), "SELECT COUNT(*) FROM vaults").Scan(&vaultCount))
	require.NoError(t, db.QueryRow(context.Background(), "SELECT COUNT(*) FROM audit_events").Scan(&auditCount))
	assert.Equal(t, 1, userCount)
	assert.Equal(t, 1, vaultCount)
	assert.GreaterOrEqual(t, auditCount, 1)

	refPayload, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	refResp := postJSON(t, server.URL+"/auth/refresh", string(refPayload))
	defer func() { _ = refResp.Body.Close() }()
	require.Equal(t, http.StatusOK, refResp.StatusCode)

	newTokens := decodeJSON(t, refResp.Body)
	newRefresh, _ := newTokens["refresh_token"].(string)
	assert.NotEmpty(t, newRefresh)

	oldRefResp := postJSON(t, server.URL+"/auth/refresh", string(refPayload))
	_ = oldRefResp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, oldRefResp.StatusCode)
}

func TestIntegration_Auth_LoginIncorretoERegistroBloqueado(t *testing.T) {
	server, _ := setupAuthIntegrationServer(t)

	firstResp := postJSON(t, server.URL+"/auth/login", `{"email":"admin@markupp.dev","password":"senhaAdmin"}`)
	_ = firstResp.Body.Close()
	require.Equal(t, http.StatusOK, firstResp.StatusCode)

	wrongPassResp := postJSON(t, server.URL+"/auth/login", `{"email":"admin@markupp.dev","password":"senhaErrada"}`)
	_ = wrongPassResp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, wrongPassResp.StatusCode)

	unknownUserResp := postJSON(t, server.URL+"/auth/login", `{"email":"intruso@markupp.dev","password":"senhaQualquer"}`)
	_ = unknownUserResp.Body.Close()
	assert.Equal(t, http.StatusForbidden, unknownUserResp.StatusCode)
}
