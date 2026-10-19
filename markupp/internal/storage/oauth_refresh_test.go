package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

// repoComTokens sobe o banco com a Autorização ativa a1 de u1 para c1 e o par
// t-acesso/t-refresh dela, que expira uma hora depois de instanteOAuth.
func repoComTokens(t *testing.T) (*storage.PostgresOAuthRepository, *pgxpool.Pool) {
	t.Helper()
	repo, pool := repoOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", false))
	gravarTokens(t, pool, tokensOAuth("a1"))
	return repo, pool
}

// gravarTokens grava os tokens direto no banco, sem passar pelo repositório.
func gravarTokens(t *testing.T, pool *pgxpool.Pool, tokens []oauth.IssuedToken) {
	t.Helper()
	for _, tk := range tokens {
		_, err := pool.Exec(context.Background(), `INSERT INTO oauth_tokens (token_hash, authorization_id, kind, resource, scopes, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6)`, tk.Hash, tk.AuthorizationID, tk.Kind, tk.Resource, tk.Scopes, tk.ExpiresAt)
		require.NoError(t, err)
	}
}

func eventoOAuth(action, result string) oauth.AuditEvent {
	return oauth.AuditEvent{Action: action, Result: result, UserID: "u1", ClientID: "c1", OccurredAt: instanteOAuth}
}

// auditoriasDe conta os eventos da ação e do resultado com u1 como ator e c1 como alvo.
func auditoriasDe(t *testing.T, pool *pgxpool.Pool, action, result string) int {
	t.Helper()
	return countRows(t, pool, `SELECT count(*) FROM audit_events WHERE action = '`+action+`'
		AND actor_id = 'u1' AND target = 'c1' AND tenant_id IS NULL AND result = '`+result+`'
		AND occurred_at = '2026-10-16T12:00:00Z'`)
}

// revogarTokensDeA1 revoga direto no banco os tokens de a1, como faz o reuso de código.
func revogarTokensDeA1(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "UPDATE oauth_tokens SET revoked = TRUE WHERE authorization_id = 'a1'")
	require.NoError(t, err)
}

func revogarAutorizacoes(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "UPDATE oauth_authorizations SET revoked_at = now()")
	require.NoError(t, err)
}

func TestPostgresOAuthRepo_ConsumeRefreshToken_RotacionaEDevolveGrant(t *testing.T) {
	repo, pool := repoComTokens(t)

	got, err := repo.ConsumeRefreshToken(context.Background(), "t-refresh", instanteOAuth)

	require.NoError(t, err)
	assert.Equal(t, "t-refresh", got.Token.Hash)
	assert.Equal(t, oauth.TokenKindRefresh, got.Token.Kind)
	assert.Equal(t, "a1", got.Token.AuthorizationID)
	assert.Equal(t, "https://mcp.markupp.dev", got.Token.Resource)
	assert.Equal(t, []string{oauth.ScopeNotesRead}, got.Token.Scopes)
	assert.True(t, instanteOAuth.Add(time.Hour).Equal(got.Token.ExpiresAt))
	assert.Equal(t, "a1", got.Authorization.ID)
	assert.Equal(t, "u1", got.Authorization.UserID)
	assert.Equal(t, "c1", got.Authorization.ClientID)
	assert.Equal(t, 1, countRows(t, pool, `SELECT count(*) FROM oauth_tokens
		WHERE token_hash = 't-refresh' AND rotated_at = '2026-10-16T12:00:00Z'`))
}

func TestPostgresOAuthRepo_ConsumeRefreshToken_RotacionadoReapresentadoEReuso(t *testing.T) {
	repo, _ := repoComTokens(t)
	ctx := context.Background()
	_, err := repo.ConsumeRefreshToken(ctx, "t-refresh", instanteOAuth)
	require.NoError(t, err)

	got, err := repo.ConsumeRefreshToken(ctx, "t-refresh", instanteOAuth.Add(time.Second))

	require.ErrorIs(t, err, oauth.ErrRefreshReused)
	assert.Equal(t, "t-refresh", got.Token.Hash)
	assert.Equal(t, "a1", got.Token.AuthorizationID)
	assert.Equal(t, "a1", got.Authorization.ID, "serve para derrubar os tokens da Autorização")
	assert.Equal(t, "c1", got.Authorization.ClientID)
}

func TestPostgresOAuthRepo_ConsumeRefreshToken_RevogadoSemRotacaoNaoEReuso(t *testing.T) {
	repo, pool := repoComTokens(t)
	ctx := context.Background()
	revogarTokensDeA1(t, pool)

	_, err := repo.ConsumeRefreshToken(ctx, "t-refresh", instanteOAuth)

	require.ErrorIs(t, err, oauth.ErrTokenNotFound)
	assert.NotErrorIs(t, err, oauth.ErrRefreshReused)
}

func TestPostgresOAuthRepo_ConsumeRefreshToken_ExpiradoNoInstanteExato(t *testing.T) {
	repo, pool := repoComTokens(t)

	_, err := repo.ConsumeRefreshToken(context.Background(), "t-refresh", instanteOAuth.Add(time.Hour))

	require.ErrorIs(t, err, oauth.ErrTokenNotFound)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE rotated_at IS NOT NULL"))
}

func TestPostgresOAuthRepo_ConsumeRefreshToken_AccessTokenNaoServe(t *testing.T) {
	repo, _ := repoComTokens(t)

	_, err := repo.ConsumeRefreshToken(context.Background(), "t-acesso", instanteOAuth)

	require.ErrorIs(t, err, oauth.ErrTokenNotFound)
}

func TestPostgresOAuthRepo_ConsumeRefreshToken_HashDesconhecidoForaDoErro(t *testing.T) {
	repo, _ := repoComTokens(t)

	_, err := repo.ConsumeRefreshToken(context.Background(), "h-segredo-desconhecido", instanteOAuth)

	require.ErrorIs(t, err, oauth.ErrTokenNotFound)
	assert.NotContains(t, err.Error(), "h-segredo-desconhecido")
}

func TestPostgresOAuthRepo_ConsumeRefreshToken_AutorizacaoRevogadaConsomeMesmoAssim(t *testing.T) {
	repo, pool := repoComTokens(t)
	ctx := context.Background()
	revogarAutorizacoes(t, pool)

	_, err := repo.ConsumeRefreshToken(ctx, "t-refresh", instanteOAuth)

	require.ErrorIs(t, err, oauth.ErrAuthorizationNotFound)
	got, err := repo.ConsumeRefreshToken(ctx, "t-refresh", instanteOAuth.Add(time.Second))
	require.ErrorIs(t, err, oauth.ErrRefreshReused, "o consumo fica gravado mesmo sem Autorização ativa")
	assert.Equal(t, "a1", got.Token.AuthorizationID)
	assert.Equal(t, "a1", got.Authorization.ID, "a Autorização revogada volta para a auditoria ter ator e alvo")
	assert.Equal(t, "u1", got.Authorization.UserID)
	assert.Equal(t, "c1", got.Authorization.ClientID)
}

func TestPostgresOAuthRepo_ConsumeRefreshToken_RotacionadoERevogadoNaoEReuso(t *testing.T) {
	repo, pool := repoComTokens(t)
	ctx := context.Background()
	_, err := repo.ConsumeRefreshToken(ctx, "t-refresh", instanteOAuth)
	require.NoError(t, err)
	revogarTokensDeA1(t, pool)

	_, err = repo.ConsumeRefreshToken(ctx, "t-refresh", instanteOAuth.Add(time.Second))

	require.ErrorIs(t, err, oauth.ErrTokenNotFound)
	assert.NotErrorIs(t, err, oauth.ErrRefreshReused, "o reuso já derrubou os tokens; reapresentar não audita de novo")
}

// parNovo é o par emitido na rotação de t-refresh, com hashes diferentes do original.
func parNovo() []oauth.IssuedToken {
	tokens := tokensOAuth("a1")
	tokens[0].Hash, tokens[1].Hash = "t-acesso-2", "t-refresh-2"
	return tokens
}

// revogarPorGatilho revoga os tokens de a1 como o reuso de refresh, disparado por gatilho.
func revogarPorGatilho(repo *storage.PostgresOAuthRepository, gatilho string, evento oauth.AuditEvent) error {
	_, err := repo.RevokeAuthorizationTokensAudited(context.Background(), "a1", gatilho, false, evento)
	return err
}

func salvarParNovo(repo *storage.PostgresOAuthRepository, tokens []oauth.IssuedToken) error {
	return repo.SaveRefreshedTokens(context.Background(), "t-refresh", tokens,
		eventoOAuth(oauth.AuditActionRefresh, oauth.AuditResultSuccess))
}

func TestPostgresOAuthRepo_SaveRefreshedTokens_GravaEAudita(t *testing.T) {
	repo, pool := repoComTokens(t)

	err := salvarParNovo(repo, parNovo())

	require.NoError(t, err)
	assert.Equal(t, 4, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE authorization_id = 'a1' AND NOT revoked"))
	assert.Equal(t, 1, auditoriasDe(t, pool, oauth.AuditActionRefresh, oauth.AuditResultSuccess))
}

func TestPostgresOAuthRepo_SaveRefreshedTokens_FalhaNaoGravaNemAudita(t *testing.T) {
	repo, pool := repoComTokens(t)
	tokens := parNovo()
	tokens[1].Scopes = []string{"notas:tudo"}

	err := salvarParNovo(repo, tokens)

	require.Error(t, err)
	assert.Equal(t, 2, countRows(t, pool, "SELECT count(*) FROM oauth_tokens"))
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM audit_events"))
}

// TestPostgresOAuthRepo_SaveRefreshedTokens_RevogacaoEntreConsumoEGravacao reproduz a
// janela em que a revogação entra entre ConsumeRefreshToken e SaveRefreshedTokens.
func TestPostgresOAuthRepo_SaveRefreshedTokens_RevogacaoEntreConsumoEGravacao(t *testing.T) {
	evento := eventoOAuth(oauth.AuditActionRevoke, oauth.AuditResultSuccess)
	casos := map[string]struct {
		revogar func(*storage.PostgresOAuthRepository) error
		want    error
	}{
		"tokens revogados por reuso ou /oauth/revoke": {
			revogar: func(r *storage.PostgresOAuthRepository) error {
				return revogarPorGatilho(r, "t-refresh", evento)
			},
			want: oauth.ErrTokenNotFound,
		},
		"autorizacao revogada pela conta": {
			revogar: func(r *storage.PostgresOAuthRepository) error {
				return r.RevokeAuthorization(context.Background(), "u1", "a1", evento)
			},
			want: oauth.ErrAuthorizationNotFound,
		},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			repo, pool := repoComTokens(t)
			_, err := repo.ConsumeRefreshToken(context.Background(), "t-refresh", instanteOAuth)
			require.NoError(t, err)
			require.NoError(t, caso.revogar(repo))

			err = salvarParNovo(repo, parNovo())

			require.ErrorIs(t, err, caso.want)
			assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE NOT revoked"))
			assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE token_hash LIKE '%-2'"))
			assert.Equal(t, 0, auditoriasDe(t, pool, oauth.AuditActionRefresh, oauth.AuditResultSuccess))
		})
	}
}

func TestPostgresOAuthRepo_ConsumeRefreshToken_ConcorrenteSoUmVence(t *testing.T) {
	repo, _ := repoComTokens(t)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := repo.ConsumeRefreshToken(context.Background(), "t-refresh", instanteOAuth)
			errs <- err
		}()
	}

	first, second := <-errs, <-errs

	if first != nil {
		first, second = second, first
	}
	require.NoError(t, first)
	require.ErrorIs(t, second, oauth.ErrRefreshReused)
}

func TestPostgresOAuthRepo_RevokeAuthorizationTokensAudited_RevogaTokensEAudita(t *testing.T) {
	repo, pool := repoComTokens(t)
	require.NoError(t, emitirTokenDe(pool, "a0", "t-outro"))

	revogou, err := repo.RevokeAuthorizationTokensAudited(context.Background(), "a1", "t-refresh", false,
		eventoOAuth(oauth.AuditActionRefreshReuse, oauth.AuditResultFailure))

	require.NoError(t, err)
	assert.True(t, revogou)
	assert.Equal(t, 2, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE authorization_id = 'a1' AND revoked"))
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE authorization_id = 'a0' AND revoked"))
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations WHERE id = 'a1' AND revoked_at IS NULL"),
		"a Autorização continua ativa")
	assert.Equal(t, 1, auditoriasDe(t, pool, oauth.AuditActionRefreshReuse, oauth.AuditResultFailure))
}

func TestPostgresOAuthRepo_FindToken_DevolveDonoMesmoRevogado(t *testing.T) {
	repo, pool := repoComTokens(t)
	ctx := context.Background()
	revogarTokensDeA1(t, pool)
	revogarAutorizacoes(t, pool)

	got, err := repo.FindToken(ctx, "t-acesso")

	require.NoError(t, err)
	assert.Equal(t, "t-acesso", got.Token.Hash)
	assert.Equal(t, oauth.TokenKindAccess, got.Token.Kind)
	assert.Equal(t, "a1", got.Authorization.ID)
	assert.Equal(t, "u1", got.Authorization.UserID)
	assert.Equal(t, "c1", got.Authorization.ClientID)
	assert.True(t, got.Token.Revoked)
	assert.True(t, got.Authorization.Revoked)
}

func TestPostgresOAuthRepo_FindToken_TokenVigente(t *testing.T) {
	repo, _ := repoComTokens(t)

	got, err := repo.FindToken(context.Background(), "t-acesso")

	require.NoError(t, err)
	assert.False(t, got.Token.Revoked)
	assert.False(t, got.Authorization.Revoked)
}

func TestPostgresOAuthRepo_FindToken_RefreshExpirado(t *testing.T) {
	repo, pool := repoComTokens(t)
	_, err := pool.Exec(context.Background(), "UPDATE oauth_tokens SET expires_at = now() - interval '1 day'")
	require.NoError(t, err)

	got, err := repo.FindToken(context.Background(), "t-refresh")

	require.NoError(t, err)
	assert.Equal(t, oauth.TokenKindRefresh, got.Token.Kind)
	assert.Equal(t, "a1", got.Authorization.ID)
}

func TestPostgresOAuthRepo_FindToken_DesconhecidoForaDoErro(t *testing.T) {
	repo, _ := repoComTokens(t)

	_, err := repo.FindToken(context.Background(), "h-segredo-desconhecido")

	require.ErrorIs(t, err, oauth.ErrTokenNotFound)
	assert.NotContains(t, err.Error(), "h-segredo-desconhecido")
}

// autorizacoesDeVariasContas grava, além de u1 e c1, a conta u2 e o cliente c2: a1 (u1, c1)
// ativa e mais antiga, a2 (u1, c2) ativa e mais recente, a0 (u1, c1) revogada e a9 de u2.
func autorizacoesDeVariasContas(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO users (id, email, created_at, updated_at) VALUES ('u2', 'u2@markupp.dev', now(), now());
		INSERT INTO oauth_clients (id, source, name, redirect_uris) VALUES ('c2', 'dcr', 'Outro', ARRAY['http://127.0.0.1/cb']);
		INSERT INTO oauth_authorizations (id, user_id, client_id, scopes, updated_at, revoked_at) VALUES
			('a1', 'u1', 'c1', '{notas:leitura}', '2026-10-01T00:00:00Z', NULL),
			('a2', 'u1', 'c2', '{notas:escrita}', '2026-10-02T00:00:00Z', NULL),
			('a0', 'u1', 'c1', '{notas:leitura}', '2026-10-03T00:00:00Z', now()),
			('a9', 'u2', 'c1', '{notas:leitura}', '2026-10-04T00:00:00Z', NULL)`)
	require.NoError(t, err)
}

func TestPostgresOAuthRepo_ListActiveAuthorizations_SoAtivasDaContaDaMaisRecente(t *testing.T) {
	repo, pool := repoOAuth(t)
	autorizacoesDeVariasContas(t, pool)

	got, err := repo.ListActiveAuthorizations(context.Background(), "u1")

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a2", got[0].ID)
	assert.Equal(t, "Outro", got[0].ClientName)
	assert.Equal(t, []string{oauth.ScopeNotesWrite}, got[0].Scopes)
	assert.Equal(t, "a1", got[1].ID)
	assert.Equal(t, "Agente", got[1].ClientName)
	assert.Equal(t, "c1", got[1].ClientID)
}

func TestPostgresOAuthRepo_ListActiveAuthorizations_ContaSemAutorizacao(t *testing.T) {
	repo, _ := repoOAuth(t)

	got, err := repo.ListActiveAuthorizations(context.Background(), "u1")

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPostgresOAuthRepo_RevokeAuthorization_RevogaAutorizacaoTokensEAuditaComOCliente(t *testing.T) {
	repo, pool := repoComTokens(t)
	evento := eventoOAuth(oauth.AuditActionRevoke, oauth.AuditResultSuccess)
	evento.ClientID = ""

	err := repo.RevokeAuthorization(context.Background(), "u1", "a1", evento)

	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, pool, `SELECT count(*) FROM oauth_authorizations
		WHERE id = 'a1' AND revoked_at = '2026-10-16T12:00:00Z'`))
	assert.Equal(t, 2, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE authorization_id = 'a1' AND revoked"))
	assert.Equal(t, 1, auditoriasDe(t, pool, oauth.AuditActionRevoke, oauth.AuditResultSuccess))
}

func TestPostgresOAuthRepo_RevokeAuthorization_DeOutraContaNaoMexe(t *testing.T) {
	repo, pool := repoComTokens(t)
	_, err := pool.Exec(context.Background(), `INSERT INTO users (id, email, created_at, updated_at)
		VALUES ('u2', 'u2@markupp.dev', now(), now())`)
	require.NoError(t, err)

	err = repo.RevokeAuthorization(context.Background(), "u2", "a1",
		eventoOAuth(oauth.AuditActionRevoke, oauth.AuditResultSuccess))

	require.ErrorIs(t, err, oauth.ErrAuthorizationNotFound)
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations WHERE id = 'a1' AND revoked_at IS NULL"))
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE revoked"))
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM audit_events"))
}

func TestPostgresOAuthRepo_RevokeAuthorization_Inexistente(t *testing.T) {
	repo, pool := repoComTokens(t)

	err := repo.RevokeAuthorization(context.Background(), "u1", "a-inexistente",
		eventoOAuth(oauth.AuditActionRevoke, oauth.AuditResultSuccess))

	require.ErrorIs(t, err, oauth.ErrAuthorizationNotFound)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM audit_events"))
}

func TestPostgresOAuthRepo_RevokeAuthorization_JaRevogada(t *testing.T) {
	repo, pool := repoComTokens(t)
	revogarAutorizacoes(t, pool)

	err := repo.RevokeAuthorization(context.Background(), "u1", "a1",
		eventoOAuth(oauth.AuditActionRevoke, oauth.AuditResultSuccess))

	require.ErrorIs(t, err, oauth.ErrAuthorizationNotFound)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM audit_events"))
}
