package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

func sessaoDe(userID, id string, at time.Time) auth.SessionRecord {
	return auth.SessionRecord{ID: id, UserID: userID, CreatedAt: at, LastUsedAt: at,
		Origin: auth.Origin{IP: "203.0.113.7", UserAgent: "Firefox/140"}}
}

// linhaDaSessao lê a sessão direto da tabela, como o administrador vai ver.
type linhaDaSessao struct {
	ip, userAgent         string
	createdAt, lastUsedAt time.Time
	ended                 bool
}

func lerSessao(t *testing.T, db *pgxpool.Pool, id string) linhaDaSessao {
	t.Helper()
	var l linhaDaSessao
	require.NoError(t, db.QueryRow(context.Background(),
		`SELECT client_ip, user_agent, created_at, last_used_at, ended_at IS NOT NULL
		 FROM user_sessions WHERE id = $1`, id).
		Scan(&l.ip, &l.userAgent, &l.createdAt, &l.lastUsedAt, &l.ended))
	return l
}

func emitirRefresh(t *testing.T, repo *storage.PostgresAuthRepository, sessao auth.SessionRecord, hash string) {
	t.Helper()
	rec := auth.RefreshTokenRecord{TokenHash: hash, UserID: sessao.UserID, SessionID: sessao.ID,
		ExpiresAt: sessao.LastUsedAt.Add(24 * time.Hour), CreatedAt: sessao.LastUsedAt}
	event := auth.AuditEventRecord{ID: "audit-" + hash, ActorID: sessao.UserID, Action: "auth.login",
		Target: sessao.UserID, Result: "success", OccurredAt: sessao.LastUsedAt}
	require.NoError(t, repo.IssueSessionTx(context.Background(), sessao, rec, event))
}

func contaComSessao(t *testing.T) (*pgxpool.Pool, *storage.PostgresAuthRepository, auth.SessionRecord) {
	t.Helper()
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	p := sampleAccountParams()
	_, err := repo.CreateAccountTx(context.Background(), p)
	require.NoError(t, err)
	sessao := sessaoDe(p.User.ID, "sessao-1", time.Now().UTC().Truncate(time.Millisecond))
	emitirRefresh(t, repo, sessao, "hash-1")
	return db, repo, sessao
}

func TestSessions_Login_GravaSessaoComOrigem(t *testing.T) {
	db, _, sessao := contaComSessao(t)

	l := lerSessao(t, db, sessao.ID)

	assert.Equal(t, "203.0.113.7", l.ip)
	assert.Equal(t, "Firefox/140", l.userAgent)
	assert.True(t, sessao.CreatedAt.Equal(l.createdAt))
	assert.False(t, l.ended)
}

func TestSessions_Renovacao_AtualizaUltimoUsoEOrigemMantemCriacao(t *testing.T) {
	db, repo, sessao := contaComSessao(t)
	renovada := sessao
	renovada.LastUsedAt = sessao.LastUsedAt.Add(time.Hour)
	renovada.CreatedAt = renovada.LastUsedAt
	renovada.Origin = auth.Origin{IP: "198.51.100.9", UserAgent: "Chrome/150"}

	emitirRefresh(t, repo, renovada, "hash-2")

	l := lerSessao(t, db, sessao.ID)
	assert.True(t, sessao.CreatedAt.Equal(l.createdAt), "a criação é a do login")
	assert.True(t, renovada.LastUsedAt.Equal(l.lastUsedAt))
	assert.Equal(t, "198.51.100.9", l.ip)
	assert.Equal(t, "Chrome/150", l.userAgent)
}

func TestSessions_Logout_EncerraSessaoERevogaOsRefreshDela(t *testing.T) {
	db, repo, sessao := contaComSessao(t)
	ctx := context.Background()
	emitirRefresh(t, repo, sessao, "hash-2")

	require.NoError(t, repo.RevokeRefreshToken(ctx, "hash-2"))

	assert.True(t, lerSessao(t, db, sessao.ID).ended)
	_, err := repo.ConsumeRefreshToken(ctx, "hash-1")
	require.ErrorIs(t, err, auth.ErrRefreshTokenInvalid, "outro refresh da mesma sessão também cai")
}

func TestSessions_RevogarTudoDaConta_EncerraTodasAsSessoes(t *testing.T) {
	db, repo, sessao := contaComSessao(t)
	outra := sessaoDe(sessao.UserID, "sessao-2", sessao.CreatedAt)
	emitirRefresh(t, repo, outra, "hash-2")

	require.NoError(t, repo.RevokeAllUserRefreshTokens(context.Background(), sessao.UserID))

	assert.True(t, lerSessao(t, db, sessao.ID).ended)
	assert.True(t, lerSessao(t, db, outra.ID).ended)
}

func TestSessions_ListarAtivas_TiraEncerradasEAsSemRefreshValido(t *testing.T) {
	db, repo, sessao := contaComSessao(t)
	ctx := context.Background()
	encerrada := sessaoDe(sessao.UserID, "sessao-encerrada", sessao.CreatedAt)
	emitirRefresh(t, repo, encerrada, "hash-encerrada")
	require.NoError(t, repo.RevokeRefreshToken(ctx, "hash-encerrada"))
	vencida := sessaoDe(sessao.UserID, "sessao-vencida", sessao.CreatedAt)
	emitirRefresh(t, repo, vencida, "hash-vencida")
	_, err := db.Exec(ctx, `UPDATE refresh_tokens SET expires_at = now() - interval '1 minute' WHERE token_hash = 'hash-vencida'`)
	require.NoError(t, err)

	ativas, err := repo.ListActiveSessions(ctx, sessao.UserID)

	require.NoError(t, err)
	require.Len(t, ativas, 1)
	assert.Equal(t, sessao.ID, ativas[0].ID)
	assert.Equal(t, sessao.Origin, ativas[0].Origin)
	assert.True(t, sessao.LastUsedAt.Equal(ativas[0].LastUsedAt))
}

func TestSessions_EncerrarDaConta_EncerraERevoga(t *testing.T) {
	db, repo, sessao := contaComSessao(t)
	ctx := context.Background()

	require.NoError(t, repo.EndUserSession(ctx, sessao.UserID, sessao.ID))

	assert.True(t, lerSessao(t, db, sessao.ID).ended)
	_, err := repo.ConsumeRefreshToken(ctx, "hash-1")
	require.ErrorIs(t, err, auth.ErrRefreshTokenInvalid)
}

func TestSessions_EncerrarDeOutraConta_NaoAcha(t *testing.T) {
	db, repo, sessao := contaComSessao(t)

	err := repo.EndUserSession(context.Background(), "outra-conta", sessao.ID)

	require.ErrorIs(t, err, auth.ErrSessionNotFound)
	assert.False(t, lerSessao(t, db, sessao.ID).ended)
}

func TestSessions_EncerrarJaEncerrada_NaoAcha(t *testing.T) {
	_, repo, sessao := contaComSessao(t)
	ctx := context.Background()
	require.NoError(t, repo.EndUserSession(ctx, sessao.UserID, sessao.ID))

	err := repo.EndUserSession(ctx, sessao.UserID, sessao.ID)

	require.ErrorIs(t, err, auth.ErrSessionNotFound)
}

func TestSessions_RefreshDeSessaoEncerrada_DizQueASessaoAcabou(t *testing.T) {
	_, repo, sessao := contaComSessao(t)
	ctx := context.Background()
	require.NoError(t, repo.EndUserSession(ctx, sessao.UserID, sessao.ID))

	rec, err := repo.GetRefreshToken(ctx, "hash-1")

	require.NoError(t, err)
	assert.True(t, rec.Revoked)
	assert.True(t, rec.SessionEnded)
}

func TestUsers_ContaBloqueada_GetUserByIDDizQueEstaBloqueada(t *testing.T) {
	db, repo, sessao := contaComSessao(t)
	_, err := db.Exec(context.Background(), `UPDATE users SET blocked_at = now() WHERE id = $1`, sessao.UserID)
	require.NoError(t, err)

	user, err := repo.GetUserByID(context.Background(), sessao.UserID)

	require.NoError(t, err)
	assert.True(t, user.Blocked)
}
