package storage_test

import (
	"context"
	"errors"
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

// entregaRegistrada faz o papel do notify.Deliverer: troca o hash pelo reissue,
// guarda o que recebeu e devolve o resultado combinado no teste.
type entregaRegistrada struct {
	outcome  notify.Outcome
	err      error
	pending  []notify.Pending
	details  notify.TokenDetails
	found    bool
	novoHash string
}

func (e *entregaRegistrada) deliver(ctx context.Context, p notify.Pending, reissue notify.ReissueFunc) (notify.Outcome, error) {
	e.pending = append(e.pending, p)
	details, found, err := reissue(ctx, e.novoHash)
	if err != nil {
		return notify.Outcome{}, err
	}
	e.details, e.found = details, found
	return e.outcome, e.err
}

func entregaProxima(t *testing.T, db *pgxpool.Pool, e *entregaRegistrada) bool {
	t.Helper()
	found, err := storage.NewPostgresInviteOutbox(db).DeliverNextDue(context.Background(), e.deliver)
	require.NoError(t, err)
	return found
}

func hashDoConvite(t *testing.T, db *pgxpool.Pool, id string) string {
	t.Helper()
	var hash string
	require.NoError(t, db.QueryRow(context.Background(), "SELECT token_hash FROM user_invites WHERE id = $1", id).Scan(&hash))
	return hash
}

func TestInviteOutbox_FilaVazia_DevolveFalso(t *testing.T) {
	db := setupTestDB(t)
	e := &entregaRegistrada{}

	assert.False(t, entregaProxima(t, db, e))
	assert.Empty(t, e.pending)
}

func TestInviteOutbox_ConviteSemEmail_NaoEntraNaFila(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, adminDoConvite, "va")

	emitirConvite(ctx, t, db, conviteDe(adminDoConvite, "c1", "nova@markupp.dev"), "tok-1")

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM notifications"))
}

func TestInviteOutbox_ConvitePorEmail_TrocaOHashEEntregaOsDetalhes(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	e := &entregaRegistrada{outcome: notify.Outcome{Sent: true}, novoHash: "hash-do-envio"}

	require.True(t, entregaProxima(t, db, e))

	require.Len(t, e.pending, 1)
	assert.Equal(t, "c1", e.pending[0].InviteID)
	assert.Zero(t, e.pending[0].Attempts)
	require.True(t, e.found)
	e.details.ExpiresAt = e.details.ExpiresAt.UTC()
	assert.Equal(t, notify.TokenDetails{
		Email: "nova@markupp.dev", Role: "member", TenantName: "t1",
		InviterEmail: "admin-a@markupp.dev", ExpiresAt: validadeDoConvite,
	}, e.details)
	assert.Equal(t, "hash-do-envio", hashDoConvite(t, db, "c1"), "o hash do token enviado substitui o da emissao")
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications WHERE sent_at IS NOT NULL"))
	assert.False(t, entregaProxima(t, db, &entregaRegistrada{}), "notificacao enviada sai da fila")
}

func TestInviteOutbox_FalhaDoSMTP_RemarcaEContaATentativa(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	retry := time.Now().Add(time.Hour)
	entregaProxima(t, db, &entregaRegistrada{outcome: notify.Outcome{RetryAt: retry, Error: "conexao recusada"}, novoHash: "h1"})

	assert.False(t, entregaProxima(t, db, &entregaRegistrada{}), "antes da hora marcada nao ha o que enviar")
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications WHERE attempts = 1 AND last_error = 'conexao recusada'"))

	_, err := db.Exec(context.Background(), "UPDATE notifications SET next_attempt_at = now()")
	require.NoError(t, err)
	e := &entregaRegistrada{outcome: notify.Outcome{Sent: true}, novoHash: "h2"}
	require.True(t, entregaProxima(t, db, e))
	assert.Equal(t, 1, e.pending[0].Attempts)
}

func TestInviteOutbox_Desistencia_FechaANotificacao(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")

	entregaProxima(t, db, &entregaRegistrada{outcome: notify.Outcome{GiveUp: true, Error: "caixa inexistente"}, novoHash: "h1"})

	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications WHERE failed_at IS NOT NULL AND attempts = 1"))
	assert.False(t, entregaProxima(t, db, &entregaRegistrada{}))
}

func TestInviteOutbox_ConviteJaRespondido_NaoTrocaOHash(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	antes := hashDoConvite(t, db, "c1")
	_, err := db.Exec(context.Background(), "UPDATE user_invites SET accepted_at = now() WHERE id = 'c1'")
	require.NoError(t, err)
	e := &entregaRegistrada{outcome: notify.Outcome{GiveUp: true}, novoHash: "h1"}

	require.True(t, entregaProxima(t, db, e))

	assert.False(t, e.found)
	assert.Equal(t, antes, hashDoConvite(t, db, "c1"))
}

func TestInviteOutbox_ConviteVencido_NaoTrocaOHash(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	antes := hashDoConvite(t, db, "c1")
	_, err := db.Exec(context.Background(), "UPDATE user_invites SET expires_at = now() - interval '1 minute' WHERE id = 'c1'")
	require.NoError(t, err)
	e := &entregaRegistrada{outcome: notify.Outcome{GiveUp: true}, novoHash: "h1"}

	require.True(t, entregaProxima(t, db, e))

	assert.False(t, e.found)
	assert.Equal(t, antes, hashDoConvite(t, db, "c1"))
}

func TestInviteOutbox_ConviteRevogado_SaiDaFila(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	ctx := identity.WithActor(context.Background(), adminDoConvite)

	require.NoError(t, storage.NewPostgresAdminRepository(db).RevokeInviteTx(ctx, "c1", eventoDoConvite(conviteDe(adminDoConvite, "rev", "x@x.dev"))))

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM notifications"))
}

func TestInviteOutbox_ErroNaEntrega_DesfazATrocaDoHash(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	antes := hashDoConvite(t, db, "c1")
	falha := errors.New("falha inesperada")
	e := &entregaRegistrada{err: falha, novoHash: "h1"}

	_, err := storage.NewPostgresInviteOutbox(db).DeliverNextDue(context.Background(), e.deliver)

	require.ErrorIs(t, err, falha)
	assert.Equal(t, antes, hashDoConvite(t, db, "c1"))
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notifications WHERE attempts = 0 AND sent_at IS NULL"))
}

func TestInviteOutbox_NotificacaoReservada_OutroEnvioPulaParaAProxima(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "um@markupp.dev")
	conviteComEmail(t, db, "c2", "dois@markupp.dev")
	outbox := storage.NewPostgresInviteOutbox(db)
	reservada := make(chan string, 1)
	libera := make(chan struct{})
	terminou := make(chan error, 1)
	go func() {
		_, err := outbox.DeliverNextDue(context.Background(), func(ctx context.Context, p notify.Pending, _ notify.ReissueFunc) (notify.Outcome, error) {
			reservada <- p.InviteID
			<-libera
			return notify.Outcome{Sent: true}, nil
		})
		terminou <- err
	}()
	primeira := <-reservada

	e := &entregaRegistrada{outcome: notify.Outcome{Sent: true}, novoHash: "h"}
	require.True(t, entregaProxima(t, db, e))
	assert.NotEqual(t, primeira, e.pending[0].InviteID, "duas réplicas nunca enviam o mesmo convite")
	assert.False(t, entregaProxima(t, db, &entregaRegistrada{}))

	close(libera)
	require.NoError(t, <-terminou)
}

func TestInviteOutbox_PendentesDoKEDA_ContaSoAsVencidas(t *testing.T) {
	db := setupTestDB(t)
	const consultaDoScaler = "SELECT markupp_pending_notifications()"
	conviteComEmail(t, db, "c1", "um@markupp.dev")
	conviteComEmail(t, db, "c2", "dois@markupp.dev")
	assert.Equal(t, 2, countRows(t, db, consultaDoScaler))

	entregaProxima(t, db, &entregaRegistrada{outcome: notify.Outcome{RetryAt: time.Now().Add(time.Hour)}, novoHash: "h"})

	assert.Equal(t, 1, countRows(t, db, consultaDoScaler), "notificacao remarcada para depois nao acorda o worker")
}

// comoPapel roda sql no papel role, sem passar pelo storage: prova a política do banco.
func comoPapel(t *testing.T, db *pgxpool.Pool, role, sql string) error {
	t.Helper()
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "SELECT set_config('role', $1, true)", role)
	require.NoError(t, err)
	var n int64
	return tx.QueryRow(ctx, sql).Scan(&n)
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

func TestRLS_PapelDoNotificador_SoTrocaOHashPelaFuncao(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")

	requirePermissaoNegada(t, comoPapel(t, db, "markupp_notifier", "SELECT count(*) FROM user_invites"), "notificador lendo convites")
	requirePermissaoNegada(t, comoPapel(t, db, "markupp_notifier",
		"WITH u AS (UPDATE user_invites SET token_hash = 'x' RETURNING 1) SELECT count(*) FROM u"), "notificador escrevendo convites")
}

// esperaConexaoTravada espera alguma conexão do banco de teste ficar aguardando trava.
func esperaConexaoTravada(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	for range 200 {
		var n int
		require.NoError(t, db.QueryRow(context.Background(),
			"SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(&n))
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.FailNow(t, "nenhuma conexao ficou aguardando trava")
}

func TestInviteOutbox_RevogarDuranteAEntrega_NaoEntraEmDeadlock(t *testing.T) {
	db := setupTestDB(t)
	conviteComEmail(t, db, "c1", "nova@markupp.dev")
	reservada, libera := make(chan struct{}), make(chan struct{})
	entrega := make(chan error, 1)
	go func() {
		_, err := storage.NewPostgresInviteOutbox(db).DeliverNextDue(context.Background(),
			func(ctx context.Context, p notify.PendingInvite, reissue notify.ReissueFunc) (notify.Outcome, error) {
				close(reservada)
				<-libera
				_, found, err := reissue(ctx, "hash-da-entrega")
				return notify.Outcome{Sent: found, GiveUp: !found}, err
			})
		entrega <- err
	}()
	<-reservada
	revogacao := make(chan error, 1)
	go func() {
		ctx := identity.WithActor(context.Background(), adminDoConvite)
		revogacao <- storage.NewPostgresAdminRepository(db).RevokeInviteTx(ctx, "c1", eventoDoConvite(conviteDe(adminDoConvite, "rev", "x@x.dev")))
	}()
	esperaConexaoTravada(t, db)

	close(libera)

	require.NoError(t, <-entrega, "o notificador desiste do convite que esta sendo revogado")
	require.NoError(t, <-revogacao)
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM user_invites WHERE id = 'c1'"))
}
