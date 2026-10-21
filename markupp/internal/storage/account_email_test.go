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

// entregaComToken reserva a próxima notificação trocando o hash pelo de token, grava
// o envio e devolve a reserva.
func entregaComToken(t *testing.T, db *pgxpool.Pool, token string) notify.Claim {
	t.Helper()
	claim, found := reservaProxima(t, db, auth.HashOpaqueToken(token))
	require.True(t, found, "esperada uma notificacao na fila")
	gravaResultado(t, db, claim, notify.Outcome{Sent: true})
	return claim
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
	claim := entregaComToken(t, db, "tok-v")

	assert.Equal(t, notify.KindEmailVerification, claim.Kind)
	require.True(t, claim.Pending)
	assert.Equal(t, "nova@markupp.dev", claim.Details.Email)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), claim.Details.ExpiresAt, time.Minute,
		"a validade de 24h conta a partir da entrega")
}

func TestAccountEmail_AutoRegistroSemVerificacao_NaoEnfileira(t *testing.T) {
	db := setupTestDB(t)

	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", false)

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM notifications"))
}

func TestAccountEmail_GetUserByID_DevolveEmailVerificado(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)

	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)

	primeira, err := repo.GetUserByID(context.Background(), "primeira")
	require.NoError(t, err)
	assert.True(t, primeira.EmailVerified, "o primeiro acesso nasce verificado")
	nova, err := repo.GetUserByID(context.Background(), "u1")
	require.NoError(t, err)
	assert.False(t, nova.EmailVerified, "o auto-registro nasce sem verificar")
}

// venceTokens empurra a validade de todo token da conta para o passado, no relógio
// do banco, que é o que grava e confere a validade.
func venceTokens(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	_, err := db.Exec(context.Background(), "UPDATE account_tokens SET expires_at = now() - interval '1 minute'")
	require.NoError(t, err)
}

// envelhecePedidos tira os pedidos da conta de dentro do intervalo entre pedidos.
func envelhecePedidos(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	_, err := db.Exec(context.Background(), "UPDATE account_tokens SET created_at = now() - interval '10 minutes'")
	require.NoError(t, err)
}

func TestAccountEmail_ConfirmEmail_TokenDoEmailVerificaAContaLogada(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAccountEmailRepository(db)
	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)
	entregaComToken(t, db, "tok-v")
	require.False(t, emailVerificado(t, db, "u1"))

	require.NoError(t, repo.ConfirmEmailTx(context.Background(), "u1", auth.HashOpaqueToken("tok-v")))

	assert.True(t, emailVerificado(t, db, "u1"))
	assert.ErrorIs(t, repo.ConfirmEmailTx(context.Background(), "u1", auth.HashOpaqueToken("tok-v")), auth.ErrInvalidToken,
		"o token vale uma vez")
}

func TestAccountEmail_ConfirmEmail_TokenDeOutraConta_NaoVerificaNemGastaOToken(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAccountEmailRepository(db)
	contaAutoRegistrada(t, db, "u1", "vitima@markupp.dev", true)
	entregaComToken(t, db, "tok-v")

	err := repo.ConfirmEmailTx(context.Background(), "primeira", auth.HashOpaqueToken("tok-v"))

	assert.ErrorIs(t, err, auth.ErrInvalidToken, "quem recebe o email nao verifica a conta de outra pessoa")
	assert.False(t, emailVerificado(t, db, "u1"))
	assert.NoError(t, repo.ConfirmEmailTx(context.Background(), "u1", auth.HashOpaqueToken("tok-v")), "a tentativa alheia nao gasta o token")
}

func TestAccountEmail_ConfirmEmail_TokenVencido_Recusa(t *testing.T) {
	db := setupTestDB(t)
	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)
	entregaComToken(t, db, "tok-v")
	venceTokens(t, db)

	err := storage.NewPostgresAccountEmailRepository(db).ConfirmEmailTx(context.Background(), "u1", auth.HashOpaqueToken("tok-v"))

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

	require.NoError(t, storage.NewPostgresAccountEmailRepository(db).ConfirmEmailTx(context.Background(), "u1", auth.HashOpaqueToken("tok-v")))

	depois, err := authRepo.ListPendingInvites(context.Background(), "u1", time.Now())
	require.NoError(t, err)
	assert.Len(t, depois, 1)
}

func TestAccountEmail_RequestEmailVerification_NovoPedidoSubstituiOAnterior(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAccountEmailRepository(db)
	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)
	entregaComToken(t, db, "tok-1")
	envelhecePedidos(t, db)

	require.NoError(t, repo.RequestEmailVerificationTx(context.Background(), "u1", cooldownDeTeste))
	entregaComToken(t, db, "tok-2")

	assert.ErrorIs(t, repo.ConfirmEmailTx(context.Background(), "u1", auth.HashOpaqueToken("tok-1")), auth.ErrInvalidToken,
		"o reenvio invalida o token anterior")
	assert.NoError(t, repo.ConfirmEmailTx(context.Background(), "u1", auth.HashOpaqueToken("tok-2")))
}

func TestAccountEmail_RequestEmailVerification_DentroDoIntervalo_NaoEnfileiraOutro(t *testing.T) {
	db := setupTestDB(t)
	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)

	require.NoError(t, storage.NewPostgresAccountEmailRepository(db).RequestEmailVerificationTx(context.Background(), "u1", cooldownDeTeste))

	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications"))
}

func TestAccountEmail_RequestEmailVerification_ContaJaVerificada_Recusa(t *testing.T) {
	db := setupTestDB(t)
	criarConta(t, db, "primeira", "primeira@markupp.dev", false)

	err := storage.NewPostgresAccountEmailRepository(db).RequestEmailVerificationTx(context.Background(), "primeira", cooldownDeTeste)

	assert.ErrorIs(t, err, auth.ErrEmailAlreadyVerified)
}

func TestAccountEmail_RequestEmailVerification_ContaApagada_RecusaComoNaoEncontrada(t *testing.T) {
	db := setupTestDB(t)

	err := storage.NewPostgresAccountEmailRepository(db).RequestEmailVerificationTx(context.Background(), "apagada", cooldownDeTeste)

	assert.ErrorIs(t, err, auth.ErrUserNotFound)
}

func TestAccountEmail_TokenTravadoPorOutraTransacao_PulaSemEsperar(t *testing.T) {
	db := setupTestDB(t)
	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", true)
	ctx := context.Background()
	reenvio, err := db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = reenvio.Rollback(ctx) }()
	_, err = reenvio.Exec(ctx, "SELECT 1 FROM account_tokens FOR UPDATE")
	require.NoError(t, err)

	claim, found := reservaProxima(t, db, "h1")

	require.True(t, found)
	assert.False(t, claim.Pending, "token em troca por reenvio e pulado, senao a entrega e o reenvio entram em deadlock")
	require.NoError(t, reenvio.Rollback(ctx))
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM account_tokens WHERE token_hash IS NOT NULL"))
}

func TestAccountEmail_RequestPasswordReset_EmailDesconhecido_NaoFazNada(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, storage.NewPostgresAccountEmailRepository(db).RequestPasswordResetTx(context.Background(), "ninguem@markupp.dev", cooldownDeTeste))

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM notifications"))
}

func TestAccountEmail_RequestPasswordReset_UmPedidoPorIntervalo(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAccountEmailRepository(db)
	criarConta(t, db, "u1", "nova@markupp.dev", false)

	require.NoError(t, repo.RequestPasswordResetTx(context.Background(), "nova@markupp.dev", cooldownDeTeste))
	require.NoError(t, repo.RequestPasswordResetTx(context.Background(), "nova@markupp.dev", cooldownDeTeste))

	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications"))
	claim := entregaComToken(t, db, "tok-r")
	assert.Equal(t, notify.KindPasswordReset, claim.Kind)
	assert.WithinDuration(t, time.Now().Add(time.Hour), claim.Details.ExpiresAt, time.Minute, "a troca de senha vale 1h a partir da entrega")
}

func TestAccountEmail_RequestPasswordReset_PedidosEmParalelo_GeramUmEmail(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAccountEmailRepository(db)
	criarConta(t, db, "u1", "nova@markupp.dev", false)
	const pedidos = 8
	erros := make(chan error, pedidos)

	for range pedidos {
		go func() {
			erros <- repo.RequestPasswordResetTx(context.Background(), "nova@markupp.dev", cooldownDeTeste)
		}()
	}
	for range pedidos {
		require.NoError(t, <-erros)
	}

	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications"), "o intervalo vale tambem para pedidos simultaneos")
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM account_tokens"))
}

func pedirTroca(t *testing.T, db *pgxpool.Pool, token string) {
	t.Helper()
	contaAutoRegistrada(t, db, "u1", "nova@markupp.dev", false)
	require.NoError(t, storage.NewPostgresAccountEmailRepository(db).RequestPasswordResetTx(context.Background(), "nova@markupp.dev", cooldownDeTeste))
	entregaComToken(t, db, token)
}

func trocaDeSenha(token string) auth.PasswordResetParams {
	return auth.PasswordResetParams{
		TokenHash: auth.HashOpaqueToken(token), AuditEventID: "ev-troca-" + token, Now: time.Now(),
		HashPassword: func() (string, error) { return "hash-novo", nil },
	}
}

func TestAccountEmail_ResetPassword_TrocaASenhaVerificaOEmailDerrubaSessoesEAudita(t *testing.T) {
	db := setupTestDB(t)
	pedirTroca(t, db, "tok-r")
	_, err := db.Exec(context.Background(), "INSERT INTO refresh_tokens (token_hash, user_id, expires_at, created_at) VALUES ('sessao', 'u1', now() + interval '1 day', now())")
	require.NoError(t, err)
	repo := storage.NewPostgresAccountEmailRepository(db)

	require.NoError(t, repo.ResetPasswordTx(context.Background(), trocaDeSenha("tok-r")))

	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM user_identities WHERE user_id = 'u1' AND password_hash = 'hash-novo'"))
	assert.True(t, emailVerificado(t, db, "u1"), "quem usou o token do email prova que controla o email")
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM refresh_tokens WHERE token_hash = 'sessao' AND revoked"))
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM audit_events WHERE action = 'auth.password_reset' AND actor_id = 'u1'"))
	assert.ErrorIs(t, repo.ResetPasswordTx(context.Background(), trocaDeSenha("tok-r")), auth.ErrInvalidToken, "o token vale uma vez")
}

func TestAccountEmail_ResetPassword_TokenInvalido_NaoCalculaOHashDaSenha(t *testing.T) {
	db := setupTestDB(t)
	calculou := false
	p := trocaDeSenha("lixo")
	p.HashPassword = func() (string, error) { calculou = true; return "x", nil }

	err := storage.NewPostgresAccountEmailRepository(db).ResetPasswordTx(context.Background(), p)

	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.False(t, calculou)
}

func TestAccountEmail_ResetPassword_TokenVencido_Recusa(t *testing.T) {
	db := setupTestDB(t)
	pedirTroca(t, db, "tok-r")
	venceTokens(t, db)

	err := storage.NewPostgresAccountEmailRepository(db).ResetPasswordTx(context.Background(), trocaDeSenha("tok-r"))

	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM user_identities WHERE user_id = 'u1' AND password_hash = 'hash-antigo'"))
}

func TestAccountEmail_TokenDeTroca_NaoVerificaEmail(t *testing.T) {
	db := setupTestDB(t)
	pedirTroca(t, db, "tok-r")

	err := storage.NewPostgresAccountEmailRepository(db).ConfirmEmailTx(context.Background(), "u1", auth.HashOpaqueToken("tok-r"))

	assert.ErrorIs(t, err, auth.ErrInvalidToken, "cada token so serve ao proprio proposito")
}

func TestRLS_PapelDoNotificador_NaoLeOsTokensDaConta(t *testing.T) {
	db := setupTestDB(t)
	pedirTroca(t, db, "tok-r")

	requirePermissaoNegada(t, comoPapel(t, db, "markupp_notifier", "SELECT count(*) FROM account_tokens"), "notificador lendo tokens da conta")
}

func TestNotificacoes_ConviteSemTenant_Recusado(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")

	_, err := db.Exec(context.Background(), "INSERT INTO notifications (tenant_id, kind, invite_id) VALUES (NULL, 'invite', 'c1')")

	require.Error(t, err, "notificacao de convite sempre carrega o tenant dele")
}
