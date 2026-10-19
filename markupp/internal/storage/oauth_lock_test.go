package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

// travarAutorizacao abre uma transação que trava a1, como fazem a gravação do par
// rotacionado e as revogações, e roda os comandos dela antes de devolvê-la aberta.
func travarAutorizacao(t *testing.T, pool *pgxpool.Pool, comandos string) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	_, err = tx.Exec(ctx, "SELECT 1 FROM oauth_authorizations WHERE id = 'a1' FOR UPDATE")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, comandos)
	require.NoError(t, err)
	return tx
}

// gravarParNovoNaTransacao é o que SaveRefreshedTokens grava depois de ver o pai vivo.
const gravarParNovoNaTransacao = `INSERT INTO oauth_tokens (token_hash, authorization_id, kind, resource, scopes, expires_at) VALUES
	('t-acesso-2', 'a1', 'access', 'https://mcp.markupp.dev', '{notas:leitura}', '2026-10-16T13:00:00Z'),
	('t-refresh-2', 'a1', 'refresh', 'https://mcp.markupp.dev', '{notas:leitura}', '2026-10-16T13:00:00Z')`

// TestPostgresOAuthRepo_Revogacao_EsperaGravacaoDoParRotacionado prova que toda revogação
// de tokens trava a Autorização: com a gravação do par segurando a trava, a revogação
// espera e, depois do commit, derruba também o par novo.
func TestPostgresOAuthRepo_Revogacao_EsperaGravacaoDoParRotacionado(t *testing.T) {
	evento := eventoOAuth(oauth.AuditActionRevoke, oauth.AuditResultSuccess)
	casos := map[string]func(*storage.PostgresOAuthRepository) error{
		"reuso de codigo": func(r *storage.PostgresOAuthRepository) error {
			return r.RevokeAuthorizationTokens(context.Background(), "a1")
		},
		"reuso de refresh ou /oauth/revoke": func(r *storage.PostgresOAuthRepository) error {
			return revogarPorGatilho(r, "t-refresh", evento)
		},
		"autorizacao revogada pela conta": func(r *storage.PostgresOAuthRepository) error {
			return r.RevokeAuthorization(context.Background(), "u1", "a1", evento)
		},
	}
	for nome, revogar := range casos {
		t.Run(nome, func(t *testing.T) {
			repo, pool := repoComTokens(t)
			gravacao := travarAutorizacao(t, pool, gravarParNovoNaTransacao)
			revogou := make(chan error, 1)
			go func() { revogou <- revogar(repo) }()
			esperarBloqueio(t, pool, "a revogação deveria esperar a trava da gravação do par rotacionado")

			require.NoError(t, gravacao.Commit(context.Background()))

			require.NoError(t, <-revogou)
			assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE NOT revoked"))
		})
	}
}

// TestPostgresOAuthRepo_SaveRefreshedTokens_EsperaRevogacao prova o outro lado: com a
// revogação segurando a trava, a gravação do par espera e falha sem gravar.
func TestPostgresOAuthRepo_SaveRefreshedTokens_EsperaRevogacao(t *testing.T) {
	repo, pool := repoComTokens(t)
	_, err := repo.ConsumeRefreshToken(context.Background(), "t-refresh", instanteOAuth)
	require.NoError(t, err)
	revogacao := travarAutorizacao(t, pool, "UPDATE oauth_tokens SET revoked = TRUE WHERE authorization_id = 'a1'")
	gravou := make(chan error, 1)
	go func() { gravou <- salvarParNovo(repo, parNovo()) }()
	esperarBloqueio(t, pool, "a gravação do par deveria esperar a trava da revogação")

	require.NoError(t, revogacao.Commit(context.Background()))

	require.ErrorIs(t, <-gravou, oauth.ErrTokenNotFound)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE token_hash LIKE '%-2'"))
	assert.Equal(t, 0, auditoriasDe(t, pool, oauth.AuditActionRefresh, oauth.AuditResultSuccess))
}
