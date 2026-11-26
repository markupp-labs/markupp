package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// contaBloqueada cria a conta e a bloqueia depois do primeiro login.
func contaBloqueada(t *testing.T) (*fakeRepository, *auth.Service, auth.TokenPair) {
	t.Helper()
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	pair, err := svc.Register(context.Background(), "admin@markupp.dev", "senha-de-teste-longa")
	require.NoError(t, err)
	for id, u := range repo.users {
		u.Blocked = true
		repo.users[id] = u
	}
	return repo, svc, pair
}

func TestBlockedAccount_SenhaCerta_RecusaComBloqueioEAudita(t *testing.T) {
	repo, svc, _ := contaBloqueada(t)

	_, err := svc.LoginLocal(context.Background(), "admin@markupp.dev", "senha-de-teste-longa")

	require.ErrorIs(t, err, auth.ErrAccountBlocked)
	evento := repo.auditEvents[len(repo.auditEvents)-1]
	assert.Equal(t, "auth.login", evento.Action)
	assert.Equal(t, "blocked", evento.Result)
}

func TestBlockedAccount_SenhaErrada_NaoRevelaOBloqueio(t *testing.T) {
	_, svc, _ := contaBloqueada(t)

	_, err := svc.LoginLocal(context.Background(), "admin@markupp.dev", "senha-errada-de-teste")

	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestBlockedAccount_Refresh_Recusa(t *testing.T) {
	_, svc, pair := contaBloqueada(t)

	_, err := svc.RefreshToken(context.Background(), pair.RefreshToken)

	require.ErrorIs(t, err, auth.ErrAccountBlocked)
}

func TestBlockedAccount_LoginExterno_Recusa(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{})
	f.withUser("user-1", githubIdentity("ident-gh", "42"))
	u := f.repo.users["user-1"]
	u.Blocked = true
	f.repo.users["user-1"] = u

	_, err := f.svc.Login(context.Background(), callback())

	require.ErrorIs(t, err, auth.ErrAccountBlocked)
}
