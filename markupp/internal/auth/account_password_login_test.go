package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func passwordLoginOf(t *testing.T, localEnabled bool) bool {
	t.Helper()
	clock := defaultClock()
	repo := newFakeRepository(clock)
	repo.users["user-1"] = auth.User{ID: "user-1", Email: "admin@markupp.dev"}
	cfg := auth.ServiceConfig{DefaultTenantID: "default", LocalEnabled: localEnabled, RefreshExpirationPeriod: time.Hour}
	svc := auth.NewService(repo, auth.NewTokenCodec("segredo-para-testes-de-servico", time.Minute, clock), &fakeHasher{}, cfg, clock)
	conta, err := svc.GetAccount(context.Background(), "user-1")
	require.NoError(t, err)
	return conta.PasswordLogin
}

func TestService_GetAccount_LoginLocalLigado_PasswordLoginTrue(t *testing.T) {
	assert.True(t, passwordLoginOf(t, true))
}

func TestService_GetAccount_LoginLocalDesligado_PasswordLoginFalse(t *testing.T) {
	assert.False(t, passwordLoginOf(t, false))
}
