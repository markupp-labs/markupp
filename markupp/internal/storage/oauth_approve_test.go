package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// esperarBloqueio espera alguma conexão do banco ficar parada esperando uma trava e
// falha com o motivo se nenhuma ficar.
func esperarBloqueio(t *testing.T, pool *pgxpool.Pool, motivo string) {
	t.Helper()
	esperarBloqueios(t, pool, 1, motivo)
}

// esperarBloqueios espera ao menos n conexões do banco ficarem paradas esperando uma
// trava e falha com o motivo se não ficarem.
func esperarBloqueios(t *testing.T, pool *pgxpool.Pool, n int, motivo string) {
	t.Helper()
	require.Eventually(t, func() bool {
		return countRows(t, pool, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`) >= n
	}, 15*time.Second, 10*time.Millisecond, motivo)
}

func TestPostgresOAuthRepo_ApproveRequest_ConcorrenteComOutraCriacaoAmplia(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r1", oauth.ScopeNotesWrite)))
	outra, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = outra.Rollback(ctx) }()
	_, err = outra.Exec(ctx, `INSERT INTO oauth_authorizations (id, user_id, client_id, scopes)
		VALUES ('a-outra', 'u1', 'c1', '{notas:leitura}')`)
	require.NoError(t, err)
	aprovou := make(chan error, 1)
	go func() {
		_, err := repo.ApproveRequest(ctx, aprovacao("r1", "h1", instanteOAuth.Add(time.Minute)))
		aprovou <- err
	}()
	esperarBloqueio(t, pool, "a aprovação deveria esperar a Autorização da outra transação")

	require.NoError(t, outra.Commit(ctx))

	require.NoError(t, <-aprovou)
	authz, err := repo.ActiveAuthorization(ctx, "u1", "c1")
	require.NoError(t, err)
	assert.Equal(t, "a-outra", authz.ID)
	assert.Equal(t, []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite}, authz.Scopes)
}

func TestPostgresOAuthRepo_ApproveRequest_CodeHashVazioRecusado(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r1", oauth.ScopeNotesRead)))

	_, err := repo.ApproveRequest(ctx, aprovacao("r1", "", instanteOAuth.Add(time.Minute)))

	require.Error(t, err)
	_, err = repo.GetPendingRequest(ctx, "r1", instanteOAuth.Add(time.Minute))
	require.NoError(t, err, "o pedido continua pendente")
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations"))
	assert.Equal(t, 0, auditoriasOAuth(t, pool, "success"))
}

// recusarAuditoriaOAuth faz o banco rejeitar qualquer auditoria de oauth.authorize.
func recusarAuditoriaOAuth(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `ALTER TABLE audit_events
		ADD CONSTRAINT recusa_oauth CHECK (action <> 'oauth.authorize') NOT VALID`)
	require.NoError(t, err)
}

func TestPostgresOAuthRepo_ApproveRequest_FalhaDeAuditoriaTemContexto(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r1", oauth.ScopeNotesRead)))
	recusarAuditoriaOAuth(t, pool)

	_, err := repo.ApproveRequest(ctx, aprovacao("r1", "h1", instanteOAuth.Add(time.Minute)))

	require.ErrorContains(t, err, `oauth.authorize`)
	require.ErrorContains(t, err, "'c1'")
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations"))
}

func TestPostgresOAuthRepo_DenyRequest_FalhaDeAuditoriaTemContexto(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r1", oauth.ScopeNotesRead)))
	recusarAuditoriaOAuth(t, pool)

	_, err := repo.DenyRequest(ctx, oauth.Denial{RequestID: "r1", UserID: "u1", DecidedAt: instanteOAuth.Add(time.Minute)})

	require.ErrorContains(t, err, `oauth.authorize`)
	require.ErrorContains(t, err, "'c1'")
}

// autorizacaoDoPedido devolve a Autorização gravada no pedido, ou "" se ele não tem.
func autorizacaoDoPedido(t *testing.T, pool *pgxpool.Pool, requestID string) string {
	t.Helper()
	var id *string
	err := pool.QueryRow(context.Background(), "SELECT authorization_id FROM oauth_requests WHERE id = $1", requestID).Scan(&id)
	require.NoError(t, err)
	if id == nil {
		return ""
	}
	return *id
}

func TestPostgresOAuthRepo_ApproveRequest_GravaNoPedidoAAutorizacaoCriadaOuAmpliada(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead)
	criada, err := repo.ActiveAuthorization(ctx, "u1", "c1")
	require.NoError(t, err)
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r2", oauth.ScopeNotesWrite)))

	_, err = repo.ApproveRequest(ctx, aprovacao("r2", "h2", instanteOAuth.Add(2*time.Minute)))

	require.NoError(t, err)
	assert.Equal(t, criada.ID, autorizacaoDoPedido(t, pool, "r1"), "o pedido que criou a Autorização")
	assert.Equal(t, criada.ID, autorizacaoDoPedido(t, pool, "r2"), "o pedido que ampliou a Autorização")
}

func TestPostgresOAuthRepo_ApproveRequest_DepoisDeRevogarGravaANova(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead)
	revogarAutorizacoes(t, pool)
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r2", oauth.ScopeNotesRead)))

	_, err := repo.ApproveRequest(ctx, aprovacao("r2", "h2", instanteOAuth.Add(2*time.Minute)))

	require.NoError(t, err)
	nova, err := repo.ActiveAuthorization(ctx, "u1", "c1")
	require.NoError(t, err)
	assert.Equal(t, nova.ID, autorizacaoDoPedido(t, pool, "r2"))
	assert.NotEqual(t, nova.ID, autorizacaoDoPedido(t, pool, "r1"))
	assert.NotEmpty(t, autorizacaoDoPedido(t, pool, "r1"), "o pedido antigo segue com a revogada")
}
