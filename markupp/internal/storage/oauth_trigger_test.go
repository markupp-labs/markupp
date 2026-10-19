package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func eventoDeRevogacao() oauth.AuditEvent {
	return eventoOAuth(oauth.AuditActionRevoke, oauth.AuditResultSuccess)
}

const revogadosDeA1 = "SELECT count(*) FROM oauth_tokens WHERE authorization_id = 'a1' AND revoked"

// prepararGatilho emite t-outro para a0 e roda o comando do caso, se houver.
func prepararGatilho(t *testing.T, pool *pgxpool.Pool, comando string) {
	t.Helper()
	require.NoError(t, emitirTokenDe(pool, "a0", "t-outro"))
	if comando == "" {
		return
	}
	_, err := pool.Exec(context.Background(), comando)
	require.NoError(t, err)
}

func TestPostgresOAuthRepo_RevokeAuthorizationTokensAudited_GatilhoQueNaoValeNaoRevoga(t *testing.T) {
	casos := map[string]struct {
		preparar         string
		gatilho          string
		requireUnexpired bool
	}{
		"gatilho desconhecido":         {gatilho: "t-desconhecido"},
		"gatilho de outra autorizacao": {gatilho: "t-outro"},
		"gatilho ja revogado":          {preparar: "UPDATE oauth_tokens SET revoked = TRUE WHERE token_hash = 't-refresh'", gatilho: "t-refresh"},
		"gatilho expirado no /oauth/revoke": {
			preparar: "UPDATE oauth_tokens SET expires_at = '2026-10-16T12:00:00Z' WHERE token_hash = 't-acesso'",
			gatilho:  "t-acesso", requireUnexpired: true,
		},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			repo, pool := repoComTokens(t)
			prepararGatilho(t, pool, caso.preparar)
			antes := countRows(t, pool, revogadosDeA1)

			revogou, err := repo.RevokeAuthorizationTokensAudited(context.Background(), "a1", caso.gatilho, caso.requireUnexpired, eventoDeRevogacao())

			require.NoError(t, err)
			assert.False(t, revogou)
			assert.Equal(t, antes, countRows(t, pool, revogadosDeA1))
			assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM audit_events"))
		})
	}
}

func TestPostgresOAuthRepo_RevokeAuthorizationTokensAudited_ReusoDeGatilhoExpiradoRevoga(t *testing.T) {
	repo, pool := repoComTokens(t)
	_, err := pool.Exec(context.Background(), "UPDATE oauth_tokens SET expires_at = '2026-10-16T12:00:00Z'")
	require.NoError(t, err)

	revogou, err := repo.RevokeAuthorizationTokensAudited(context.Background(), "a1", "t-refresh", false, eventoDeRevogacao())

	require.NoError(t, err)
	assert.True(t, revogou)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE NOT revoked"))
}

func TestPostgresOAuthRepo_RevokeAuthorizationTokensAudited_ReapresentadoAuditaUmaVez(t *testing.T) {
	repo, pool := repoComTokens(t)
	ctx := context.Background()
	primeira, err := repo.RevokeAuthorizationTokensAudited(ctx, "a1", "t-refresh", false, eventoDeRevogacao())
	require.NoError(t, err)

	segunda, err := repo.RevokeAuthorizationTokensAudited(ctx, "a1", "t-refresh", false, eventoDeRevogacao())

	require.NoError(t, err)
	assert.True(t, primeira)
	assert.False(t, segunda, "o gatilho já foi revogado pela primeira")
	assert.Equal(t, 1, auditoriasDe(t, pool, oauth.AuditActionRevoke, oauth.AuditResultSuccess))
}

// TestPostgresOAuthRepo_RevokeAuthorizationTokensAudited_EsperaOutraRevogacao prova que a
// conferência do gatilho acontece com a trava: com outra revogação segurando a1, esta
// espera e, depois do commit, vê o gatilho revogado e não revoga nem audita.
func TestPostgresOAuthRepo_RevokeAuthorizationTokensAudited_EsperaOutraRevogacao(t *testing.T) {
	casos := map[string]bool{"reuso de refresh": false, "/oauth/revoke": true}
	for nome, requireUnexpired := range casos {
		t.Run(nome, func(t *testing.T) {
			repo, pool := repoComTokens(t)
			outra := travarAutorizacao(t, pool, "UPDATE oauth_tokens SET revoked = TRUE WHERE authorization_id = 'a1'")
			revogou := make(chan bool, 1)
			go func() {
				ok, err := repo.RevokeAuthorizationTokensAudited(context.Background(), "a1", "t-refresh", requireUnexpired, eventoDeRevogacao())
				assert.NoError(t, err)
				revogou <- ok
			}()
			esperarBloqueio(t, pool, "a revogação deveria esperar a trava da outra revogação")

			require.NoError(t, outra.Commit(context.Background()))

			assert.False(t, <-revogou)
			assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM audit_events"))
		})
	}
}
