package storage_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

var instanteOAuth = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// janelaReusoCodigo é a janela, contada da decisão do pedido, em que o reuso do código
// ainda revoga os tokens; os testes usam a duração padrão do pedido.
const janelaReusoCodigo = 10 * time.Minute

// repoOAuth sobe o banco com a conta u1 e o cliente c1 e devolve o repositório sobre ele.
func repoOAuth(t *testing.T) (*storage.PostgresOAuthRepository, *pgxpool.Pool) {
	t.Helper()
	pool := bancoComOAuth(t)
	return storage.NewPostgresOAuthRepository(pool), pool
}

func pedidoOAuth(id string, escopos ...string) oauth.AuthorizationRequest {
	return oauth.AuthorizationRequest{
		ID: id, ClientID: "c1", RedirectURI: "http://127.0.0.1/cb", Scopes: escopos,
		Resource: "https://mcp.markupp.dev", State: "estado-" + id, CodeChallenge: "desafio-" + id,
		CreatedAt: instanteOAuth, ExpiresAt: instanteOAuth.Add(10 * time.Minute),
	}
}

func aprovacao(requestID, codeHash string, decidedAt time.Time) oauth.Approval {
	return oauth.Approval{
		RequestID: requestID, UserID: "u1", CodeHash: codeHash,
		CodeExpiresAt: decidedAt.Add(time.Minute), DecidedAt: decidedAt,
	}
}

// pedidoAprovado grava o pedido e o aprova um minuto depois da criação.
func pedidoAprovado(t *testing.T, repo *storage.PostgresOAuthRepository, id, codeHash string, escopos ...string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth(id, escopos...)))
	_, err := repo.ApproveRequest(ctx, aprovacao(id, codeHash, instanteOAuth.Add(time.Minute)))
	require.NoError(t, err)
}

func auditoriasOAuth(t *testing.T, pool *pgxpool.Pool, result string) int {
	t.Helper()
	return countRows(t, pool, `SELECT count(*) FROM audit_events WHERE action = 'oauth.authorize'
		AND actor_id = 'u1' AND target = 'c1' AND tenant_id IS NULL AND result = '`+result+`'`)
}

func TestPostgresOAuthRepo_GetClient_DevolveCliente(t *testing.T) {
	repo, _ := repoOAuth(t)

	client, err := repo.GetClient(context.Background(), "c1", time.Time{})

	require.NoError(t, err)
	assert.Equal(t, oauth.Client{ID: "c1", Name: "Agente", RedirectURIs: []string{"http://127.0.0.1/cb"}}, client)
}

func TestPostgresOAuthRepo_GetClient_Inexistente(t *testing.T) {
	repo, _ := repoOAuth(t)

	_, err := repo.GetClient(context.Background(), "c-nenhum", time.Time{})

	require.ErrorIs(t, err, oauth.ErrClientNotFound)
}

func TestPostgresOAuthRepo_ClientIDLongo_ChegaLimitadoAoErro(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	longo := strings.Repeat("c", 500)
	pedido := pedidoOAuth("r1", oauth.ScopeNotesRead)
	pedido.ClientID = longo
	casos := map[string]func() error{
		"cliente inexistente":           func() error { _, err := repo.GetClient(ctx, longo, time.Time{}); return err },
		"pedido de cliente inexistente": func() error { return repo.CreateRequest(ctx, pedido) },
		"autorização inexistente":       func() error { _, err := repo.ActiveAuthorization(ctx, "u1", longo); return err },
	}
	for nome, falha := range casos {
		t.Run(nome, func(t *testing.T) {
			err := falha()

			require.Error(t, err)
			assert.NotContains(t, err.Error(), longo)
		})
	}
}

func TestPostgresOAuthRepo_CreateRequest_PendenteVoltaIgual(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	pedido := pedidoOAuth("r1", oauth.ScopeNotesRead, oauth.ScopeAdmin)
	require.NoError(t, repo.CreateRequest(ctx, pedido))

	got, err := repo.GetPendingRequest(ctx, "r1", instanteOAuth.Add(time.Minute))

	require.NoError(t, err)
	assert.Equal(t, pedido.Scopes, got.Scopes)
	assert.Equal(t, pedido.State, got.State)
	assert.Equal(t, pedido.CodeChallenge, got.CodeChallenge)
	assert.Equal(t, pedido.Resource, got.Resource)
	assert.True(t, pedido.ExpiresAt.Equal(got.ExpiresAt))
	assert.Empty(t, got.UserID)
}

func TestPostgresOAuthRepo_GetPendingRequest_Inexistente(t *testing.T) {
	repo, _ := repoOAuth(t)

	_, err := repo.GetPendingRequest(context.Background(), "r-nenhum", instanteOAuth)

	require.ErrorIs(t, err, oauth.ErrRequestNotFound)
}

func TestPostgresOAuthRepo_GetPendingRequest_ExpiradoNoInstanteExato(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	pedido := pedidoOAuth("r1", oauth.ScopeNotesRead)
	require.NoError(t, repo.CreateRequest(ctx, pedido))

	_, err := repo.GetPendingRequest(ctx, "r1", pedido.ExpiresAt)

	require.ErrorIs(t, err, oauth.ErrRequestNotFound)
}

func TestPostgresOAuthRepo_GetPendingRequest_JaDecidido(t *testing.T) {
	repo, _ := repoOAuth(t)
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead)

	_, err := repo.GetPendingRequest(context.Background(), "r1", instanteOAuth.Add(2*time.Minute))

	require.ErrorIs(t, err, oauth.ErrRequestNotFound)
}

func TestPostgresOAuthRepo_ApproveRequest_CriaAutorizacaoEAudita(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r1", oauth.ScopeNotesRead)))
	decidedAt := instanteOAuth.Add(time.Minute)

	got, err := repo.ApproveRequest(ctx, aprovacao("r1", "h1", decidedAt))

	require.NoError(t, err)
	assert.Equal(t, "u1", got.UserID)
	assert.Equal(t, "c1", got.ClientID)
	authz, err := repo.ActiveAuthorization(ctx, "u1", "c1")
	require.NoError(t, err)
	assert.Equal(t, []string{oauth.ScopeNotesRead}, authz.Scopes)
	assert.NotEmpty(t, authz.ID)
	assert.True(t, decidedAt.Equal(authz.CreatedAt))
	assert.Equal(t, 1, auditoriasOAuth(t, pool, "success"))
}

func TestPostgresOAuthRepo_ApproveRequest_DuplaAprovacaoRecusada(t *testing.T) {
	repo, pool := repoOAuth(t)
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead)

	_, err := repo.ApproveRequest(context.Background(), aprovacao("r1", "h2", instanteOAuth.Add(2*time.Minute)))

	require.ErrorIs(t, err, oauth.ErrRequestNotFound)
	assert.Equal(t, 1, auditoriasOAuth(t, pool, "success"))
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_requests WHERE code_hash = 'h2'"))
}

func TestPostgresOAuthRepo_ApproveRequest_ExpiradoRecusado(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	pedido := pedidoOAuth("r1", oauth.ScopeNotesRead)
	require.NoError(t, repo.CreateRequest(ctx, pedido))

	_, err := repo.ApproveRequest(ctx, aprovacao("r1", "h1", pedido.ExpiresAt))

	require.ErrorIs(t, err, oauth.ErrRequestNotFound)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations"))
	assert.Equal(t, 0, auditoriasOAuth(t, pool, "success"))
}

func TestPostgresOAuthRepo_ApproveRequest_InexistenteRecusado(t *testing.T) {
	repo, _ := repoOAuth(t)

	_, err := repo.ApproveRequest(context.Background(), aprovacao("r-nenhum", "h1", instanteOAuth))

	require.ErrorIs(t, err, oauth.ErrRequestNotFound)
}

func TestPostgresOAuthRepo_ApproveRequest_UneEscoposNaOrdemSuportada(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeAdmin)
	primeira, err := repo.ActiveAuthorization(ctx, "u1", "c1")
	require.NoError(t, err)
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r2", oauth.ScopeNotesWrite, oauth.ScopeNotesRead)))
	segundaDecisao := instanteOAuth.Add(3 * time.Minute)

	_, err = repo.ApproveRequest(ctx, aprovacao("r2", "h2", segundaDecisao))

	require.NoError(t, err)
	authz, err := repo.ActiveAuthorization(ctx, "u1", "c1")
	require.NoError(t, err)
	assert.Equal(t, primeira.ID, authz.ID, "amplia a Autorização existente em vez de criar outra")
	assert.Equal(t, []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite, oauth.ScopeAdmin}, authz.Scopes)
	assert.True(t, segundaDecisao.Equal(authz.UpdatedAt))
	assert.True(t, primeira.CreatedAt.Equal(authz.CreatedAt))
	assert.Equal(t, 2, auditoriasOAuth(t, pool, "success"))
}

func TestPostgresOAuthRepo_ApproveRequest_UneEscopoRepetidoNosDoisLados(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead, oauth.ScopeNotesWrite)
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r2", oauth.ScopeNotesWrite, oauth.ScopeAdmin)))

	_, err := repo.ApproveRequest(ctx, aprovacao("r2", "h2", instanteOAuth.Add(3*time.Minute)))

	require.NoError(t, err)
	authz, err := repo.ActiveAuthorization(ctx, "u1", "c1")
	require.NoError(t, err)
	assert.Equal(t, []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite, oauth.ScopeAdmin}, authz.Scopes)
}

func TestPostgresOAuthRepo_ApproveRequest_DepoisDeRevogarCriaNova(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeAdmin)
	revogada, err := repo.ActiveAuthorization(ctx, "u1", "c1")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE oauth_authorizations SET revoked_at = now()")
	require.NoError(t, err)
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r2", oauth.ScopeNotesRead)))

	_, err = repo.ApproveRequest(ctx, aprovacao("r2", "h2", instanteOAuth.Add(3*time.Minute)))

	require.NoError(t, err)
	authz, err := repo.ActiveAuthorization(ctx, "u1", "c1")
	require.NoError(t, err)
	assert.NotEqual(t, revogada.ID, authz.ID)
	assert.Equal(t, []string{oauth.ScopeNotesRead}, authz.Scopes, "não herda os escopos da revogada")
	assert.Equal(t, 2, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations"))
}

func TestPostgresOAuthRepo_DenyRequest_DecideEAuditaFalha(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r1", oauth.ScopeNotesRead)))
	denial := oauth.Denial{RequestID: "r1", UserID: "u1", DecidedAt: instanteOAuth.Add(time.Minute)}

	got, err := repo.DenyRequest(ctx, denial)

	require.NoError(t, err)
	assert.Equal(t, "u1", got.UserID)
	assert.Equal(t, 1, auditoriasOAuth(t, pool, "failure"))
	_, err = repo.ApproveRequest(ctx, aprovacao("r1", "h1", instanteOAuth.Add(2*time.Minute)))
	require.ErrorIs(t, err, oauth.ErrRequestNotFound, "pedido recusado não pode ser aprovado depois")
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations"))
}

func TestPostgresOAuthRepo_DenyRequest_ExpiradoRecusado(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	pedido := pedidoOAuth("r1", oauth.ScopeNotesRead)
	require.NoError(t, repo.CreateRequest(ctx, pedido))

	_, err := repo.DenyRequest(ctx, oauth.Denial{RequestID: "r1", UserID: "u1", DecidedAt: pedido.ExpiresAt})

	require.ErrorIs(t, err, oauth.ErrRequestNotFound)
	assert.Equal(t, 0, auditoriasOAuth(t, pool, "failure"))
}

func TestPostgresOAuthRepo_DenyRequest_DuplaRecusaRecusada(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r1", oauth.ScopeNotesRead)))
	denial := oauth.Denial{RequestID: "r1", UserID: "u1", DecidedAt: instanteOAuth.Add(time.Minute)}
	_, err := repo.DenyRequest(ctx, denial)
	require.NoError(t, err)

	_, err = repo.DenyRequest(ctx, denial)

	require.ErrorIs(t, err, oauth.ErrRequestNotFound)
	assert.Equal(t, 1, auditoriasOAuth(t, pool, "failure"))
}

func TestPostgresOAuthRepo_DenyRequest_InexistenteRecusado(t *testing.T) {
	repo, pool := repoOAuth(t)

	_, err := repo.DenyRequest(context.Background(), oauth.Denial{RequestID: "r-nenhum", UserID: "u1", DecidedAt: instanteOAuth})

	require.ErrorIs(t, err, oauth.ErrRequestNotFound)
	assert.Equal(t, 0, auditoriasOAuth(t, pool, "failure"))
}

func TestPostgresOAuthRepo_ActiveAuthorization_Inexistente(t *testing.T) {
	repo, _ := repoOAuth(t)

	_, err := repo.ActiveAuthorization(context.Background(), "u1", "c1")

	require.ErrorIs(t, err, oauth.ErrAuthorizationNotFound)
}

func TestPostgresOAuthRepo_ActiveAuthorization_IgnoraRevogada(t *testing.T) {
	repo, pool := repoOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", true))

	_, err := repo.ActiveAuthorization(context.Background(), "u1", "c1")

	require.ErrorIs(t, err, oauth.ErrAuthorizationNotFound)
}

func TestPostgresOAuthRepo_RedeemCode_DevolvePedidoEAutorizacao(t *testing.T) {
	repo, _ := repoOAuth(t)
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead)

	got, err := repo.RedeemCode(context.Background(), "h1", instanteOAuth.Add(90*time.Second), janelaReusoCodigo)

	require.NoError(t, err)
	assert.Equal(t, "r1", got.Request.ID)
	assert.Equal(t, "u1", got.Request.UserID)
	assert.Equal(t, "desafio-r1", got.Request.CodeChallenge)
	assert.Equal(t, "u1", got.Authorization.UserID)
	assert.Equal(t, "c1", got.Authorization.ClientID)
	assert.NotEmpty(t, got.Authorization.ID)
}

func TestPostgresOAuthRepo_RedeemCode_ReusoDevolveOriginalComErro(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead)
	primeiro, err := repo.RedeemCode(ctx, "h1", instanteOAuth.Add(90*time.Second), janelaReusoCodigo)
	require.NoError(t, err)

	got, err := repo.RedeemCode(ctx, "h1", instanteOAuth.Add(100*time.Second), janelaReusoCodigo)

	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
	assert.Equal(t, "r1", got.Request.ID)
	assert.Equal(t, primeiro.Authorization.ID, got.Authorization.ID, "serve para revogar os tokens da Autorização")
}

func TestPostgresOAuthRepo_RedeemCode_ReusoDepoisDeExpirarContinuaJaUsado(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead)
	_, err := repo.RedeemCode(ctx, "h1", instanteOAuth.Add(90*time.Second), janelaReusoCodigo)
	require.NoError(t, err)

	got, err := repo.RedeemCode(ctx, "h1", instanteOAuth.Add(time.Hour), janelaReusoCodigo)

	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
	assert.Equal(t, "r1", got.Request.ID)
}

func TestPostgresOAuthRepo_RedeemCode_ReusoComAutorizacaoRevogadaDevolveADoPedido(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead)
	primeiro, err := repo.RedeemCode(ctx, "h1", instanteOAuth.Add(90*time.Second), janelaReusoCodigo)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE oauth_authorizations SET revoked_at = now()")
	require.NoError(t, err)

	got, err := repo.RedeemCode(ctx, "h1", instanteOAuth.Add(100*time.Second), janelaReusoCodigo)

	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
	assert.Equal(t, "r1", got.Request.ID)
	assert.Equal(t, "u1", got.Request.UserID)
	assert.Equal(t, primeiro.Authorization.ID, got.Authorization.ID, "a Autorização do pedido, mesmo revogada")
	assert.True(t, got.Authorization.Revoked)
}

func TestPostgresOAuthRepo_RedeemCode_ExpiradoNoInstanteExato(t *testing.T) {
	repo, _ := repoOAuth(t)
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead)
	codeExpiresAt := instanteOAuth.Add(2 * time.Minute)

	_, err := repo.RedeemCode(context.Background(), "h1", codeExpiresAt, janelaReusoCodigo)

	require.ErrorIs(t, err, oauth.ErrCodeNotFound)
}

func TestPostgresOAuthRepo_RedeemCode_HashDesconhecido(t *testing.T) {
	repo, _ := repoOAuth(t)

	_, err := repo.RedeemCode(context.Background(), "h-nenhum", instanteOAuth, janelaReusoCodigo)

	require.ErrorIs(t, err, oauth.ErrCodeNotFound)
}

func TestPostgresOAuthRepo_RedeemCode_SemAutorizacaoAtiva(t *testing.T) {
	repo, pool := repoOAuth(t)
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead)
	_, err := pool.Exec(context.Background(), "UPDATE oauth_authorizations SET revoked_at = now()")
	require.NoError(t, err)

	_, err = repo.RedeemCode(context.Background(), "h1", instanteOAuth.Add(90*time.Second), janelaReusoCodigo)

	require.ErrorIs(t, err, oauth.ErrAuthorizationNotFound)
	_, err = repo.RedeemCode(context.Background(), "h1", instanteOAuth.Add(100*time.Second), janelaReusoCodigo)
	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed, "o consumo fica gravado mesmo sem Autorização ativa")
}

func tokensOAuth(authorizationID string) []oauth.IssuedToken {
	expira := instanteOAuth.Add(time.Hour)
	return []oauth.IssuedToken{
		{Hash: "t-acesso", AuthorizationID: authorizationID, Kind: oauth.TokenKindAccess,
			Resource: "https://mcp.markupp.dev", Scopes: []string{oauth.ScopeNotesRead}, ExpiresAt: expira},
		{Hash: "t-refresh", AuthorizationID: authorizationID, Kind: oauth.TokenKindRefresh,
			Resource: "https://mcp.markupp.dev", Scopes: []string{oauth.ScopeNotesRead}, ExpiresAt: expira},
	}
}

// repoComPedidoDeA1 sobe o banco com a Autorização ativa a1 e o pedido r1 ligado a ela.
func repoComPedidoDeA1(t *testing.T) (*storage.PostgresOAuthRepository, *pgxpool.Pool) {
	t.Helper()
	repo, pool := repoOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", false))
	require.NoError(t, pedidoDaAutorizacao(pool, "a1"))
	return repo, pool
}

func TestPostgresOAuthRepo_SaveCodeTokens_GravaTodosComEscopos(t *testing.T) {
	repo, pool := repoComPedidoDeA1(t)

	require.NoError(t, repo.SaveCodeTokens(context.Background(), "r1", tokensOAuth("a1")))

	assert.Equal(t, 2, countRows(t, pool, `SELECT count(*) FROM oauth_tokens
		WHERE authorization_id = 'a1' AND scopes = '{notas:leitura}' AND NOT revoked`))
}

func TestPostgresOAuthRepo_SaveCodeTokens_FalhaNaoGravaNenhum(t *testing.T) {
	repo, pool := repoComPedidoDeA1(t)
	tokens := tokensOAuth("a1")
	tokens[1].Scopes = []string{"notas:tudo"}

	require.Error(t, repo.SaveCodeTokens(context.Background(), "r1", tokens))

	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens"))
}

// emitirTokenDe grava um token numa Autorização revogada a0, para conferir o alcance da revogação.
func emitirTokenDe(pool *pgxpool.Pool, authorizationID, hash string) error {
	if err := autorizar(pool, authorizationID, "{notas:leitura}", true); err != nil {
		return err
	}
	_, err := pool.Exec(context.Background(), `INSERT INTO oauth_tokens (token_hash, authorization_id, kind, resource, scopes, expires_at)
		VALUES ($1, $2, 'access', 'https://mcp.markupp.dev', '{notas:leitura}', now() + interval '1 hour')`, hash, authorizationID)
	return err
}

func TestPostgresOAuthRepo_RecordAuditEvent_GravaSemTenantComContaECliente(t *testing.T) {
	repo, pool := repoOAuth(t)

	err := repo.RecordAuditEvent(context.Background(), oauth.AuditEvent{
		Action: oauth.AuditActionExchange, Result: oauth.AuditResultSuccess,
		UserID: "u1", ClientID: "c1", OccurredAt: instanteOAuth,
	})

	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, pool, `SELECT count(*) FROM audit_events WHERE action = 'oauth.exchange'
		AND result = 'success' AND actor_id = 'u1' AND target = 'c1' AND tenant_id IS NULL
		AND occurred_at = '2026-10-16T12:00:00Z'`))
}
