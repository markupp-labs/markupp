package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func (f *fakeAuthService) ListMyNotifications(ctx context.Context, userID string) (auth.NotificationFeed, error) {
	f.notificationsUser = userID
	return f.notifications, f.notificationsErr
}

// notificationsRequest pede GET path; com token, a conta é u1.
func notificationsRequest(svc *fakeAuthService, path string, withToken bool) *httptest.ResponseRecorder {
	router := setupAuthRouter(svc, &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}})
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if withToken {
		req.Header.Set("Authorization", "Bearer token-valido")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestMyNotifications_Lista_DevolveOsItensDaContaComOTipo(t *testing.T) {
	expira := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	svc := &fakeAuthService{notifications: auth.NotificationFeed{Unread: 1, Items: []auth.Notification{
		{Kind: auth.NotificationKindInvite, ID: "c1", TenantID: "t2", TenantName: "Home lab", Role: "member", ExpiresAt: expira},
	}}}

	rec := notificationsRequest(svc, "/v1/me/notifications", true)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"items":[{"kind":"invite","id":"c1","tenant_id":"t2","tenant_name":"Home lab","role":"member","expires_at":"2026-11-01T12:00:00Z"}],"unread":1}`, rec.Body.String())
	assert.Equal(t, "u1", svc.notificationsUser)
}

func TestMyNotifications_SemNotificacao_DevolveListaVazia(t *testing.T) {
	rec := notificationsRequest(&fakeAuthService{}, "/v1/me/notifications", true)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"items":[],"unread":0}`, rec.Body.String())
}

func TestMyNotifications_SemToken_Retorna401(t *testing.T) {
	rec := notificationsRequest(&fakeAuthService{}, "/v1/me/notifications", false)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMyNotifications_NasceuDepoisDaTransicao_NaoTemAliasSemPrefixo(t *testing.T) {
	rec := notificationsRequest(&fakeAuthService{}, "/me/notifications", true)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMyNotifications_ErroInesperado_RegistraComMensagemDasNotificacoes(t *testing.T) {
	buf := logCapturado(t)

	rec := notificationsRequest(&fakeAuthService{notificationsErr: errors.New("banco fora")}, "/v1/me/notifications", true)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, buf.String(), `"msg":"falha nas notificacoes da conta"`)
	assert.Contains(t, buf.String(), "banco fora")
}
