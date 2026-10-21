package auth_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// fakeAccountEmailRepository faz o papel do banco nos pedidos de email da conta:
// guarda o que recebeu e devolve err. Na troca de senha, só chama o hash da senha
// quando o token vale, como o banco faz.
type fakeAccountEmailRepository struct {
	err error

	verificationFor string
	confirmedUser   string
	confirmedHash   string
	resetEmail      string
	resetCooldown   time.Duration
	reset           auth.PasswordResetParams
	resetHash       string
}

func (f *fakeAccountEmailRepository) RequestEmailVerificationTx(_ context.Context, userID string, _ time.Duration) error {
	f.verificationFor = userID
	return f.err
}

func (f *fakeAccountEmailRepository) ConfirmEmailTx(_ context.Context, userID, tokenHash string) error {
	f.confirmedUser, f.confirmedHash = userID, tokenHash
	return f.err
}

func (f *fakeAccountEmailRepository) RequestPasswordResetTx(_ context.Context, email string, cooldown time.Duration) error {
	f.resetEmail, f.resetCooldown = email, cooldown
	return f.err
}

func (f *fakeAccountEmailRepository) ResetPasswordTx(_ context.Context, p auth.PasswordResetParams) error {
	f.reset = p
	if f.err != nil {
		return f.err
	}
	hash, err := p.HashPassword()
	f.resetHash = hash
	return err
}

// backgroundSincrono roda o trabalho na hora, para o teste ver o efeito.
func backgroundSincrono(f func()) { f() }

func opcoesDeTeste() auth.AccountEmailOptions {
	return auth.AccountEmailOptions{EmailEnabled: true, LocalEnabled: true, Background: backgroundSincrono, Logger: slog.New(slog.DiscardHandler)}
}

func novoAccountEmail(repo auth.AccountEmailRepository, opts auth.AccountEmailOptions) *auth.AccountEmailService {
	return auth.NewAccountEmailService(repo, &fakeHasher{}, defaultClock(), opts)
}

func TestAccountEmail_SemSMTP_TodoPedidoDeEmailFicaIndisponivel(t *testing.T) {
	opts := opcoesDeTeste()
	opts.EmailEnabled = false
	svc := novoAccountEmail(&fakeAccountEmailRepository{}, opts)
	ctx := context.Background()

	assert.ErrorIs(t, svc.RequestEmailVerification(ctx, "u1"), auth.ErrEmailUnavailable)
	assert.ErrorIs(t, svc.RequestPasswordReset(ctx, "a@markupp.dev"), auth.ErrEmailUnavailable)
}

func TestAccountEmail_SemCredencialLocal_TrocaDeSenhaFicaIndisponivel(t *testing.T) {
	opts := opcoesDeTeste()
	opts.LocalEnabled = false
	repo := &fakeAccountEmailRepository{}
	svc := novoAccountEmail(repo, opts)
	ctx := context.Background()

	assert.ErrorIs(t, svc.RequestPasswordReset(ctx, "a@markupp.dev"), auth.ErrLocalAuthDisabled)
	assert.ErrorIs(t, svc.ResetPassword(ctx, "tok", "senhaNova123"), auth.ErrLocalAuthDisabled)
	assert.Empty(t, repo.resetEmail+repo.reset.TokenHash, "nada chega ao banco")
}

func TestAccountEmail_RequestEmailVerification_EnfileiraParaAConta(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	require.NoError(t, novoAccountEmail(repo, opcoesDeTeste()).RequestEmailVerification(context.Background(), "u1"))

	assert.Equal(t, "u1", repo.verificationFor)
}

func TestAccountEmail_RequestEmailVerification_ContaJaVerificada_Recusa(t *testing.T) {
	repo := &fakeAccountEmailRepository{err: auth.ErrEmailAlreadyVerified}

	err := novoAccountEmail(repo, opcoesDeTeste()).RequestEmailVerification(context.Background(), "u1")

	assert.ErrorIs(t, err, auth.ErrEmailAlreadyVerified)
}

func TestAccountEmail_ConfirmEmail_EntregaAContaLogadaESoOHashDoToken(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	require.NoError(t, novoAccountEmail(repo, opcoesDeTeste()).ConfirmEmail(context.Background(), "u1", "tok-1"))

	assert.Equal(t, "u1", repo.confirmedUser, "o token so verifica a conta logada")
	assert.Equal(t, auth.HashOpaqueToken("tok-1"), repo.confirmedHash)
}

func TestAccountEmail_ConfirmEmail_TokenVazio_RecusaSemIrAoBanco(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	err := novoAccountEmail(repo, opcoesDeTeste()).ConfirmEmail(context.Background(), "u1", "  ")

	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.Empty(t, repo.confirmedHash)
}

func TestAccountEmail_RequestPasswordReset_NormalizaOEmailEAplicaOIntervalo(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	require.NoError(t, novoAccountEmail(repo, opcoesDeTeste()).RequestPasswordReset(context.Background(), "  Nova@Markupp.dev "))

	assert.Equal(t, "nova@markupp.dev", repo.resetEmail)
	assert.Equal(t, 5*time.Minute, repo.resetCooldown, "um pedido por conta a cada 5 minutos contra envio em massa")
}

func TestAccountEmail_RequestPasswordReset_RodaForaDaRequisicaoESemDevolverErroDoBanco(t *testing.T) {
	repo := &fakeAccountEmailRepository{err: errors.New("banco fora")}
	var adiado func()
	opts := opcoesDeTeste()
	opts.Background = func(f func()) { adiado = f }
	svc := novoAccountEmail(repo, opts)

	err := svc.RequestPasswordReset(context.Background(), "nova@markupp.dev")

	require.NoError(t, err, "a resposta sai antes de gravar, igual exista a conta ou nao")
	assert.Empty(t, repo.resetEmail, "nada foi gravado durante a requisicao")
	require.NotNil(t, adiado)
	adiado()
	assert.Equal(t, "nova@markupp.dev", repo.resetEmail)
}

func TestAccountEmail_RequestPasswordReset_EmailVazio_Recusa(t *testing.T) {
	err := novoAccountEmail(&fakeAccountEmailRepository{}, opcoesDeTeste()).RequestPasswordReset(context.Background(), " ")

	assert.ErrorIs(t, err, auth.ErrEmptyEmail)
}

func TestAccountEmail_ResetPassword_GravaOHashDaSenhaNovaEAudita(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	require.NoError(t, novoAccountEmail(repo, opcoesDeTeste()).ResetPassword(context.Background(), "tok-1", "senhaNova123"))

	assert.Equal(t, auth.HashOpaqueToken("tok-1"), repo.reset.TokenHash)
	assert.Equal(t, "hash:senhaNova123", repo.resetHash)
	assert.NotEmpty(t, repo.reset.AuditEventID)
	assert.Equal(t, defaultClock()(), repo.reset.Now)
}

func TestAccountEmail_ResetPassword_TokenInvalido_NaoGastaBcrypt(t *testing.T) {
	hasher := &countingHasher{}
	svc := auth.NewAccountEmailService(&fakeAccountEmailRepository{err: auth.ErrInvalidToken}, hasher, defaultClock(), opcoesDeTeste())

	err := svc.ResetPassword(context.Background(), "lixo", "senhaNova123")

	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.Zero(t, hasher.ops, "token lixo nao custa um bcrypt")
}

func TestAccountEmail_ResetPassword_SenhaVazia_RecusaSemIrAoBanco(t *testing.T) {
	repo := &fakeAccountEmailRepository{}

	err := novoAccountEmail(repo, opcoesDeTeste()).ResetPassword(context.Background(), "tok-1", " ")

	assert.ErrorIs(t, err, auth.ErrEmptyPassword)
	assert.Empty(t, repo.reset.TokenHash)
}
