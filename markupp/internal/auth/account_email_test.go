package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// fakeAccountEmailRepository faz o papel do banco nos pedidos de email da conta:
// guarda o que recebeu e devolve err.
type fakeAccountEmailRepository struct {
	err error

	verificationFor string
	confirmedHash   string
	resetEmail      string
	resetCooldown   time.Duration
	reset           auth.PasswordResetParams
	requestedAt     time.Time
}

func (f *fakeAccountEmailRepository) RequestEmailVerificationTx(_ context.Context, userID string, now time.Time, cooldown time.Duration) error {
	f.verificationFor, f.requestedAt = userID, now
	return f.err
}

func (f *fakeAccountEmailRepository) ConfirmEmailTx(_ context.Context, tokenHash string, now time.Time) error {
	f.confirmedHash, f.requestedAt = tokenHash, now
	return f.err
}

func (f *fakeAccountEmailRepository) RequestPasswordResetTx(_ context.Context, email string, now time.Time, cooldown time.Duration) error {
	f.resetEmail, f.resetCooldown, f.requestedAt = email, cooldown, now
	return f.err
}

func (f *fakeAccountEmailRepository) ResetPasswordTx(_ context.Context, p auth.PasswordResetParams) error {
	f.reset = p
	return f.err
}

func novoAccountEmail(repo auth.AccountEmailRepository, emailEnabled bool) *auth.AccountEmailService {
	return auth.NewAccountEmailService(repo, &fakeHasher{}, defaultClock(), emailEnabled)
}

func TestAccountEmail_SemSMTP_TodoPedidoDeEmailFicaIndisponivel(t *testing.T) {
	svc := novoAccountEmail(&fakeAccountEmailRepository{}, false)
	ctx := context.Background()

	assert.ErrorIs(t, svc.RequestEmailVerification(ctx, "u1"), auth.ErrEmailUnavailable)
	assert.ErrorIs(t, svc.RequestPasswordReset(ctx, "a@markupp.dev"), auth.ErrEmailUnavailable)
}

func TestAccountEmail_RequestEmailVerification_EnfileiraParaAConta(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	require.NoError(t, novoAccountEmail(repo, true).RequestEmailVerification(context.Background(), "u1"))

	assert.Equal(t, "u1", repo.verificationFor)
	assert.Equal(t, defaultClock()(), repo.requestedAt)
}

func TestAccountEmail_RequestEmailVerification_ContaJaVerificada_Recusa(t *testing.T) {
	repo := &fakeAccountEmailRepository{err: auth.ErrEmailAlreadyVerified}

	err := novoAccountEmail(repo, true).RequestEmailVerification(context.Background(), "u1")

	assert.ErrorIs(t, err, auth.ErrEmailAlreadyVerified)
}

func TestAccountEmail_ConfirmEmail_EntregaSoOHashDoToken(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	require.NoError(t, novoAccountEmail(repo, true).ConfirmEmail(context.Background(), "tok-1"))

	assert.Equal(t, auth.HashOpaqueToken("tok-1"), repo.confirmedHash)
}

func TestAccountEmail_ConfirmEmail_TokenVazio_RecusaSemIrAoBanco(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	err := novoAccountEmail(repo, true).ConfirmEmail(context.Background(), "  ")

	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.Empty(t, repo.confirmedHash)
}

func TestAccountEmail_RequestPasswordReset_NormalizaOEmailEAplicaOIntervalo(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	require.NoError(t, novoAccountEmail(repo, true).RequestPasswordReset(context.Background(), "  Nova@Markupp.dev "))

	assert.Equal(t, "nova@markupp.dev", repo.resetEmail)
	assert.Equal(t, 5*time.Minute, repo.resetCooldown, "um pedido por conta a cada 5 minutos contra envio em massa")
}

func TestAccountEmail_RequestPasswordReset_EmailVazio_Recusa(t *testing.T) {
	err := novoAccountEmail(&fakeAccountEmailRepository{}, true).RequestPasswordReset(context.Background(), " ")

	assert.ErrorIs(t, err, auth.ErrEmptyEmail)
}

func TestAccountEmail_ResetPassword_GravaOHashDaSenhaNovaEAudita(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	require.NoError(t, novoAccountEmail(repo, true).ResetPassword(context.Background(), "tok-1", "senhaNova123"))

	assert.Equal(t, auth.HashOpaqueToken("tok-1"), repo.reset.TokenHash)
	assert.Equal(t, "hash:senhaNova123", repo.reset.PasswordHash)
	assert.NotEmpty(t, repo.reset.AuditEventID)
	assert.Equal(t, defaultClock()(), repo.reset.Now)
}

func TestAccountEmail_ResetPassword_SenhaVazia_RecusaSemIrAoBanco(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	err := novoAccountEmail(repo, true).ResetPassword(context.Background(), "tok-1", " ")

	assert.ErrorIs(t, err, auth.ErrEmptyPassword)
	assert.Empty(t, repo.reset.TokenHash)
}

func TestAccountEmail_ResetPassword_TokenInvalido_Recusa(t *testing.T) {
	err := novoAccountEmail(&fakeAccountEmailRepository{err: auth.ErrInvalidToken}, true).ResetPassword(context.Background(), "tok", "senhaNova123")

	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}
