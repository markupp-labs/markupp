package auth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func TestRegister_SenhaCurta_RecusaAntesDoBanco(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)

	_, err := setupAuthService(repo, true, clock).Register(context.Background(), "ana@markupp.dev", "catorze-chars!")

	require.ErrorIs(t, err, auth.ErrPasswordTooShort)
	assert.Empty(t, repo.users)
}

func TestRegister_QuinzeCaracteresAcentuados_Aceita(t *testing.T) {
	clock := defaultClock()

	_, err := setupAuthService(newFakeRepository(clock), true, clock).Register(context.Background(), "ana@markupp.dev", strings.Repeat("ç", 15))

	require.NoError(t, err, "o minimo conta caracteres, nao bytes")
}

func TestRegister_SenhaComum_RecusaSemDiferenciarCaixa(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)

	_, err := setupAuthService(repo, true, clock).Register(context.Background(), "ana@markupp.dev", "QAZWSXEDCRFVTGB")

	require.ErrorIs(t, err, auth.ErrCommonPassword)
	assert.Empty(t, repo.users)
}

func TestAcceptInvite_SenhaCurta_NaoConsomeOConvite(t *testing.T) {
	clock := defaultClock()
	repo := repoComConvite(clock, "token-do-convite")

	_, err := setupAuthService(repo, false, clock).AcceptInvite(context.Background(), "token-do-convite", "curta")

	require.ErrorIs(t, err, auth.ErrPasswordTooShort)
	assert.Len(t, repo.pendingInvites, 1)
}

func TestAccountEmail_ResetPassword_SenhaComum_RecusaSemIrAoBanco(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	err := novoAccountEmail(repo, opcoesDeTeste()).ResetPassword(context.Background(), "tok-1", "1q2w3e4r5t6y7u8i")

	require.ErrorIs(t, err, auth.ErrCommonPassword)
	assert.Empty(t, repo.reset.TokenHash)
}

func TestLoginLocal_SenhaCurtaDeContaAntiga_ContinuaEntrando(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, true, clock)
	_, err := svc.Register(context.Background(), "ana@markupp.dev", "senha-de-teste-longa")
	require.NoError(t, err)
	for id, identity := range repo.identities {
		identity.PasswordHash = "hash:curta"
		repo.identities[id] = identity
	}

	_, err = svc.LoginLocal(context.Background(), "ana@markupp.dev", "curta")

	require.NoError(t, err, "a regra vale para senha nova, o login de conta antiga segue")
}
