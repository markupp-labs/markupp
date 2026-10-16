package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notify"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

const cooldownDeTeste = 5 * time.Minute

// criarConta cria a conta email com identidade local pelo repositório de autenticação.
// A primeira conta do banco nasce verificada (primeiro acesso); as outras, por
// auto-registro, nascem sem verificar.
func criarConta(t *testing.T, db *pgxpool.Pool, id, email string, sendVerification bool) {
	t.Helper()
	agora := time.Now()
	_, err := storage.NewPostgresAuthRepository(db).CreateAccountTx(context.Background(), auth.CreateAccountParams{
		User:              auth.User{ID: id, Email: email, CreatedAt: agora, UpdatedAt: agora},
		BootstrapTenantID: "default",
		Identity: auth.UserIdentity{
			ID: "ident-" + id, UserID: id, Provider: "local", ProviderUserID: email,
			PasswordHash: "hash-antigo", CreatedAt: agora,
		},
		InitialVaultID: "cofre-" + id, InitialVaultName: auth.InitialVaultName,
		AllowMemberRegistration: true, AuditEventID: "ev-" + id, SendVerification: sendVerification,
	})
	require.NoError(t, err)
}

// contaAutoRegistrada cria o primeiro acesso e depois a conta id por auto-registro.
func contaAutoRegistrada(t *testing.T, db *pgxpool.Pool, id, email string, sendVerification bool) {
	t.Helper()
	criarConta(t, db, "primeira", "primeira@markupp.dev", false)
	criarConta(t, db, id, email, sendVerification)
}

// entregaComToken entrega a próxima notificação trocando o hash pelo de token e
// devolve o que a entrega recebeu.
func entregaComToken(t *testing.T, db *pgxpool.Pool, token string) *entregaRegistrada {
	t.Helper()
	e := &entregaRegistrada{outcome: notify.Outcome{Sent: true}, novoHash: auth.HashOpaqueToken(token)}
	require.True(t, entregaProxima(t, db, e), "esperada uma notificacao na fila")
	return e
}

func emailVerificado(t *testing.T, db *pgxpool.Pool, id string) bool {
	t.Helper()
	var ok bool
	require.NoError(t, db.QueryRow(context.Background(), "SELECT email_verified FROM users WHERE id = $1", id).Scan(&ok))
	return ok
}

func TestAccountEmail_AutoRegistroComVerificacao_EnfileiraOEmail(t *testing.T) {
	db := setupTestDB(t)

	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)
	e := entregaComToken(t, db, "tok-v")

	assert.Equal(t, notify.KindEmailVerification, e.pending[0].Kind)
	require.True(t, e.found)
	assert.Equal(t, "nova@markupp.dev", e.details.Email)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), e.details.ExpiresAt, time.Minute,
		"a validade de 24h conta a partir da entrega")
}

func TestAccountEmail_AutoRegistroSemVerificacao_NaoEnfileira(t *testing.T) {
	db := setupTestDB(t)

	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", false)

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM notifications"))
}

func TestAccountEmail_ConfirmEmail_TokenDoEmailVerificaAConta(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAccountEmailRepository(db)
	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)
	entregaComToken(t, db, "tok-v")
	require.False(t, emailVerificado(t, db, "u1"))

	require.NoError(t, repo.ConfirmEmailTx(context.Background(), auth.HashOpaqueToken("tok-v"), time.Now()))

	assert.True(t, emailVerificado(t, db, "u1"))
	assert.ErrorIs(t, repo.ConfirmEmailTx(context.Background(), auth.HashOpaqueToken("tok-v"), time.Now()), auth.ErrInvalidToken,
		"o token vale uma vez")
}

func TestAccountEmail_ConfirmEmail_TokenVencido_Recusa(t *testing.T) {
	db := setupTestDB(t)
	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)
	entregaComToken(t, db, "tok-v")

	err := storage.NewPostgresAccountEmailRepository(db).ConfirmEmailTx(context.Background(), auth.HashOpaqueToken("tok-v"), time.Now().Add(25*time.Hour))

	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.False(t, emailVerificado(t, db, "u1"))
}

func TestAccountEmail_ContaVerificada_PassaAVerOsConvites(t *testing.T) {
	db := setupTestDB(t)
	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)
	entregaComToken(t, db, "tok-v")
	ctx := storagetest.SeedMember(t, db, adminDoConvite, "va")
	inv := conviteDe(adminDoConvite, "c1", "nova@markupp.dev")
	inv.ExpiresAt = validadeDoConvite
	require.NoError(t, storage.NewPostgresAdminRepository(db).CreateInviteTx(ctx, inv, auth.HashOpaqueToken("tok-c"), eventoDoConvite(inv), false))
	authRepo := storage.NewPostgresAuthRepository(db)
	antes, err := authRepo.ListPendingInvites(context.Background(), "u1", time.Now())
	require.NoError(t, err)
	require.Empty(t, antes, "conta sem verificar fica fora dos convites")

	require.NoError(t, storage.NewPostgresAccountEmailRepository(db).ConfirmEmailTx(context.Background(), auth.HashOpaqueToken("tok-v"), time.Now()))

	depois, err := authRepo.ListPendingInvites(context.Background(), "u1", time.Now())
	require.NoError(t, err)
	assert.Len(t, depois, 1)
}

func TestAccountEmail_RequestEmailVerification_NovoPedidoSubstituiOAnterior(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAccountEmailRepository(db)
	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)
	entregaComToken(t, db, "tok-1")

	require.NoError(t, repo.RequestEmailVerificationTx(context.Background(), "u1", time.Now().Add(cooldownDeTeste+time.Second), cooldownDeTeste))
	entregaComToken(t, db, "tok-2")

	assert.ErrorIs(t, repo.ConfirmEmailTx(context.Background(), auth.HashOpaqueToken("tok-1"), time.Now()), auth.ErrInvalidToken,
		"o reenvio invalida o token anterior")
	assert.NoError(t, repo.ConfirmEmailTx(context.Background(), auth.HashOpaqueToken("tok-2"), time.Now()))
}

func TestAccountEmail_RequestEmailVerification_DentroDoIntervalo_NaoEnfileiraOutro(t *testing.T) {
	db := setupTestDB(t)
	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)

	require.NoError(t, storage.NewPostgresAccountEmailRepository(db).RequestEmailVerificationTx(context.Background(), "u1", time.Now(), cooldownDeTeste))

	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications"))
}

func TestAccountEmail_RequestEmailVerification_ContaJaVerificada_Recusa(t *testing.T) {
	db := setupTestDB(t)
	criarConta(t, db, "primeira", "primeira@markupp.dev", false)

	err := storage.NewPostgresAccountEmailRepository(db).RequestEmailVerificationTx(context.Background(), "primeira", time.Now(), cooldownDeTeste)

	assert.ErrorIs(t, err, auth.ErrEmailAlreadyVerified)
}

func TestAccountEmail_RequestPasswordReset_EmailDesconhecido_NaoFazNada(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, storage.NewPostgresAccountEmailRepository(db).RequestPasswordResetTx(context.Background(), "ninguem@markupp.dev", time.Now(), cooldownDeTeste))

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM notifications"))
}

func TestAccountEmail_RequestPasswordReset_UmPedidoPorIntervalo(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAccountEmailRepository(db)
	criarConta(t, db, "u1", "nova@markupp.dev", false)

	require.NoError(t, repo.RequestPasswordResetTx(context.Background(), "nova@markupp.dev", time.Now(), cooldownDeTeste))
	require.NoError(t, repo.RequestPasswordResetTx(context.Background(), "nova@markupp.dev", time.Now(), cooldownDeTeste))

	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications"))
	e := entregaComToken(t, db, "tok-r")
	assert.Equal(t, notify.KindPasswordReset, e.pending[0].Kind)
	assert.WithinDuration(t, time.Now().Add(time.Hour), e.details.ExpiresAt, time.Minute, "a troca de senha vale 1h a partir da entrega")
}

func pedirTroca(t *testing.T, db *pgxpool.Pool, token string) {
	t.Helper()
	criarConta(t, db, "u1", "nova@markupp.dev", false)
	require.NoError(t, storage.NewPostgresAccountEmailRepository(db).RequestPasswordResetTx(context.Background(), "nova@markupp.dev", time.Now(), cooldownDeTeste))
	entregaComToken(t, db, token)
}

func trocaDeSenha(token string, now time.Time) auth.PasswordResetParams {
	return auth.PasswordResetParams{TokenHash: auth.HashOpaqueToken(token), PasswordHash: "hash-novo", AuditEventID: "ev-troca-" + token, Now: now}
}

func TestAccountEmail_ResetPassword_TrocaASenhaDerrubaSessoesEAudita(t *testing.T) {
	db := setupTestDB(t)
	pedirTroca(t, db, "tok-r")
	_, err := db.Exec(context.Background(), "INSERT INTO refresh_tokens (token_hash, user_id, expires_at, created_at) VALUES ('sessao', 'u1', now() + interval '1 day', now())")
	require.NoError(t, err)
	repo := storage.NewPostgresAccountEmailRepository(db)

	require.NoError(t, repo.ResetPasswordTx(context.Background(), trocaDeSenha("tok-r", time.Now())))

	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM user_identities WHERE user_id = 'u1' AND password_hash = 'hash-novo'"))
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM refresh_tokens WHERE token_hash = 'sessao' AND revoked"))
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM audit_events WHERE action = 'auth.password_reset' AND actor_id = 'u1'"))
	assert.ErrorIs(t, repo.ResetPasswordTx(context.Background(), trocaDeSenha("tok-r", time.Now())), auth.ErrInvalidToken, "o token vale uma vez")
}

func TestAccountEmail_ResetPassword_TokenVencido_Recusa(t *testing.T) {
	db := setupTestDB(t)
	pedirTroca(t, db, "tok-r")

	err := storage.NewPostgresAccountEmailRepository(db).ResetPasswordTx(context.Background(), trocaDeSenha("tok-r", time.Now().Add(2*time.Hour)))

	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM user_identities WHERE user_id = 'u1' AND password_hash = 'hash-antigo'"))
}

func TestRLS_PapelDoNotificador_NaoLeOsPedidosDaConta(t *testing.T) {
	db := setupTestDB(t)
	pedirTroca(t, db, "tok-r")

	requirePermissaoNegada(t, comoPapel(t, db, "markupp_notifier", "SELECT count(*) FROM password_resets"), "notificador lendo trocas de senha")
	requirePermissaoNegada(t, comoPapel(t, db, "markupp_notifier", "SELECT count(*) FROM email_verifications"), "notificador lendo verificacoes")
}
