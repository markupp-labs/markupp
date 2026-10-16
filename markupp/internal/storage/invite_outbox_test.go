package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notify"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

var adminDoConvite = identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}

// validadeDoConvite fica no futuro: a troca do hash recusa convite vencido.
var validadeDoConvite = time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)

// conviteComEmail semeia o admin e emite por ele, com envio por email, o convite id.
func conviteComEmail(t *testing.T, db *pgxpool.Pool, id, email string) {
	t.Helper()
	ctx := storagetest.SeedMember(t, db, adminDoConvite, "va")
	inv := conviteDe(adminDoConvite, id, email)
	inv.ExpiresAt = validadeDoConvite
	require.NoError(t, storage.NewPostgresAdminRepository(db).CreateInviteTx(ctx, inv, auth.HashOpaqueToken("descartado-"+id), eventoDoConvite(inv), true))
}

// reservaProxima reserva a próxima notificação vencida por uma hora, trocando o hash
// do convite por hash.
func reservaProxima(t *testing.T, db *pgxpool.Pool, hash string) (notify.Claim, bool) {
	t.Helper()
	claim, found, err := storage.NewPostgresInviteOutbox(db).ClaimNextDue(context.Background(), hash, time.Hour)
	require.NoError(t, err)
	return claim, found
}

func gravaResultado(t *testing.T, db *pgxpool.Pool, claim notify.Claim, outcome notify.Outcome) {
	t.Helper()
	require.NoError(t, storage.NewPostgresInviteOutbox(db).Record(context.Background(), claim.NotificationID, outcome))
}

func venceAsNotificacoes(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	_, err := db.Exec(context.Background(), "UPDATE notifications SET next_attempt_at = now()")
	require.NoError(t, err)
}

func hashDoConvite(t *testing.T, db *pgxpool.Pool, id string) string {
	t.Helper()
	var hash string
	require.NoError(t, db.QueryRow(context.Background(), "SELECT token_hash FROM user_invites WHERE id = $1", id).Scan(&hash))
	return hash
}

func TestInviteOutbox_FilaVazia_DevolveFalso(t *testing.T) {
	db := setupTestDB(t)

	_, found := reservaProxima(t, db, "h")

	assert.False(t, found)
}

func TestInviteOutbox_ConviteSemEmail_NaoEntraNaFila(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, adminDoConvite, "va")

	emitirConvite(ctx, t, db, conviteDe(adminDoConvite, "c1", "nova@markupp.dev"), "tok-1")

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM notifications"))
}

func TestInviteOutbox_Reserva_TrocaOHashJaGravadoEDevolveOsDetalhes(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")

	claim, found := reservaProxima(t, db, "hash-do-envio")

	require.True(t, found)
	assert.Equal(t, 1, claim.Attempt)
	require.True(t, claim.Pending)
	claim.Details.ExpiresAt = claim.Details.ExpiresAt.UTC()
	assert.Equal(t, notify.InviteDetails{
		Email: "nova@markupp.dev", Role: "member", TenantName: "t1",
		InviterEmail: "admin-a@markupp.dev", ExpiresAt: validadeDoConvite,
	}, claim.Details)
	assert.Equal(t, "hash-do-envio", hashDoConvite(t, db, "c1"), "o hash ja esta gravado antes do envio, fora de transacao")
}

func TestInviteOutbox_Enviada_SaiDaFila(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	claim, _ := reservaProxima(t, db, "h1")

	gravaResultado(t, db, claim, notify.Outcome{Sent: true})
	venceAsNotificacoes(t, db)

	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications WHERE sent_at IS NOT NULL"))
	_, found := reservaProxima(t, db, "h2")
	assert.False(t, found, "notificacao enviada sai da fila")
}

func TestInviteOutbox_Reservada_SomeDaFilaAteOLeaseVencer(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	reservaProxima(t, db, "h1")

	_, found := reservaProxima(t, db, "h2")
	assert.False(t, found, "duas replicas nunca enviam o mesmo convite durante o lease")
	assert.Equal(t, "h1", hashDoConvite(t, db, "c1"))

	venceAsNotificacoes(t, db)
	claim, found := reservaProxima(t, db, "h3")
	require.True(t, found, "notificador que caiu no meio do envio devolve a notificacao quando o lease vence")
	assert.Equal(t, 2, claim.Attempt, "a tentativa conta na reserva, e a queda tambem gasta uma")
}

func TestInviteOutbox_FalhaDoSMTP_RemarcaComOErro(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	claim, _ := reservaProxima(t, db, "h1")

	gravaResultado(t, db, claim, notify.Outcome{RetryAt: time.Now().Add(-time.Second), Error: "conexao recusada"})

	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications WHERE attempts = 1 AND last_error = 'conexao recusada'"))
	proxima, found := reservaProxima(t, db, "h2")
	require.True(t, found, "a remarcacao substitui o lease")
	assert.Equal(t, 2, proxima.Attempt)
}

func TestInviteOutbox_ErroComBytesForaDoUTF8_GravaMesmoAssim(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	claim, _ := reservaProxima(t, db, "h1")

	err := storage.NewPostgresInviteOutbox(db).Record(context.Background(), claim.NotificationID,
		notify.Outcome{RetryAt: time.Now().Add(time.Hour), Error: "550 Usu\xe1rio inexistente\x00"})

	require.NoError(t, err, "a resposta do relay em Latin-1 nao pode travar a fila")
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications WHERE last_error LIKE '550 Usu%rio inexistente'"))
}

func TestInviteOutbox_Desistencia_FechaANotificacao(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	claim, _ := reservaProxima(t, db, "h1")

	gravaResultado(t, db, claim, notify.Outcome{GiveUp: true, Error: "caixa inexistente"})
	venceAsNotificacoes(t, db)

	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications WHERE failed_at IS NOT NULL AND attempts = 1"))
	_, found := reservaProxima(t, db, "h2")
	assert.False(t, found)
}

// conviteForaDoEnvio emite o convite c1 por email, aplica mudanca no banco e confere
// que a reserva não troca o hash nem pede o envio.
func conviteForaDoEnvio(t *testing.T, mudanca string) {
	t.Helper()
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	antes := hashDoConvite(t, db, "c1")
	_, err := db.Exec(context.Background(), mudanca)
	require.NoError(t, err)

	claim, found := reservaProxima(t, db, "h1")

	require.True(t, found)
	assert.False(t, claim.Pending)
	assert.Equal(t, antes, hashDoConvite(t, db, "c1"))
}

func TestInviteOutbox_ConviteJaRespondido_NaoTrocaOHash(t *testing.T) {
	conviteForaDoEnvio(t, "UPDATE user_invites SET accepted_at = now() WHERE id = 'c1'")
}

func TestInviteOutbox_ConviteVencido_NaoTrocaOHash(t *testing.T) {
	conviteForaDoEnvio(t, "UPDATE user_invites SET expires_at = now() - interval '1 minute' WHERE id = 'c1'")
}

func TestInviteOutbox_QuemConvidouSaiuDoTenant_NaoTrocaOHash(t *testing.T) {
	conviteForaDoEnvio(t, "DELETE FROM tenant_memberships WHERE user_id = 'admin-a'")
}

func TestInviteOutbox_ConviteRevogado_SaiDaFila(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	ctx := identity.WithActor(context.Background(), adminDoConvite)

	require.NoError(t, storage.NewPostgresAdminRepository(db).RevokeInviteTx(ctx, "c1", eventoDoConvite(conviteDe(adminDoConvite, "rev", "x@x.dev"))))

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM notifications"))
}

func TestInviteOutbox_PendentesDoKEDA_ContaSoAsVencidas(t *testing.T) {
	db := setupTestDB(t)
	const consultaDoScaler = "SELECT markupp_pending_notifications()"
	conviteComEmail(t, db, "c1", "um@markupp.dev")
	conviteComEmail(t, db, "c2", "dois@markupp.dev")
	assert.Equal(t, 2, countRows(t, db, consultaDoScaler))

	reservaProxima(t, db, "h")

	assert.Equal(t, 1, countRows(t, db, consultaDoScaler), "notificacao reservada ou remarcada para depois nao acorda o worker")
}

func requirePermissaoNegada(t *testing.T, err error, contexto string) {
	t.Helper()
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr, contexto)
	assert.Equal(t, "42501", pgErr.Code, "%s: esperada permissao negada, recebido %v", contexto, err)
}

func TestRLS_PapelDaAplicacao_NaoLeAFila(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")

	requirePermissaoNegada(t, comoPapel(t, db, "markupp_app", "SELECT count(*) FROM notifications"), "aplicacao lendo a fila")
}

func TestPendentesDoKEDA_ScalerContaANotificacaoVencida(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, comoPapel(t, db, "markupp_scaler", "SELECT markupp_pending_notifications()"),
		"o KEDA conta a fila de email como markupp_scaler, sem a URL do dono do banco")
	requirePermissaoNegada(t, comoPapel(t, db, "markupp_app", "SELECT markupp_pending_notifications()"), "aplicacao contando a fila")
}

func TestRLS_PapelDoNotificador_SoTrocaOHashPelaFuncao(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")

	requirePermissaoNegada(t, comoPapel(t, db, "markupp_notifier", "SELECT count(*) FROM user_invites"), "notificador lendo convites")
	requirePermissaoNegada(t, comoPapel(t, db, "markupp_notifier",
		"WITH u AS (UPDATE user_invites SET token_hash = 'x' RETURNING 1) SELECT count(*) FROM u"), "notificador escrevendo convites")
}

func TestRLS_PapelDaAplicacao_NaoEnfileiraConviteDeOutroTenant(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "admin-b", TenantID: "t2", Role: "admin"}, "vb")
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "SELECT set_config('markupp.tenant_id', 't2', true), set_config('role', 'markupp_app', true)")
	require.NoError(t, err)

	_, err = tx.Exec(ctx, "INSERT INTO notifications (invite_id) VALUES ('c1')")

	requirePermissaoNegada(t, err, "tenant t2 enfileirando o convite do t1")
}

func TestInviteOutbox_ConviteTravadoPorOutraTransacao_PulaSemEsperar(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	antes := hashDoConvite(t, db, "c1")
	ctx := context.Background()
	revogacao, err := db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = revogacao.Rollback(ctx) }()
	_, err = revogacao.Exec(ctx, "SELECT 1 FROM user_invites WHERE id = 'c1' FOR UPDATE")
	require.NoError(t, err)

	claim, found := reservaProxima(t, db, "h1")

	require.True(t, found)
	assert.False(t, claim.Pending, "convite em revogacao e pulado, senao a entrega e a revogacao entram em deadlock")
	require.NoError(t, revogacao.Rollback(ctx))
	assert.Equal(t, antes, hashDoConvite(t, db, "c1"))
}
