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

// esperarBloqueio espera alguma conexão do banco ficar parada esperando uma trava.
func esperarBloqueio(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	require.Eventually(t, func() bool {
		return countRows(t, pool, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`) > 0
	}, 5*time.Second, 10*time.Millisecond, "a aprovação deveria esperar a Autorização da outra transação")
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
	esperarBloqueio(t, pool)

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
