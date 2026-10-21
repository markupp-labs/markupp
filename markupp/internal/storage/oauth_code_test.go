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

// repoComCodigoDeA1 sobe o banco com a Autorização ativa a1 e o pedido r1 aprovado por u1,
// ligado a ela, com o código h1 ainda não usado, que expira dois minutos depois de
// instanteOAuth.
func repoComCodigoDeA1(t *testing.T) (*storage.PostgresOAuthRepository, *pgxpool.Pool) {
	t.Helper()
	repo, pool := repoComPedidoDeA1(t)
	_, err := pool.Exec(context.Background(), `UPDATE oauth_requests SET user_id = 'u1', decided_at = $1,
		code_hash = 'h1', code_expires_at = $2 WHERE id = 'r1'`, instanteOAuth.Add(time.Minute), instanteOAuth.Add(2*time.Minute))
	require.NoError(t, err)
	return repo, pool
}

// trocarCodigo consome h1 como a primeira troca do código.
func trocarCodigo(t *testing.T, repo *storage.PostgresOAuthRepository) oauth.RedeemedCode {
	t.Helper()
	redeemed, err := repo.RedeemCode(context.Background(), "h1", instanteOAuth.Add(90*time.Second), janelaReusoCodigo)
	require.NoError(t, err)
	return redeemed
}

// reusarCodigo reapresenta h1 depois da primeira troca.
func reusarCodigo(repo *storage.PostgresOAuthRepository, at time.Time) (oauth.RedeemedCode, error) {
	return repo.RedeemCode(context.Background(), "h1", at, janelaReusoCodigo)
}

func tokensAtivos(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	return countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE NOT revoked")
}

func TestPostgresOAuthRepo_RedeemCode_LeAAutorizacaoDoPedidoENaoAAtivaDoPar(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	pedidoAprovado(t, repo, "r1", "h1", oauth.ScopeNotesRead)
	revogarAutorizacoes(t, pool)
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth("r2", oauth.ScopeNotesRead)))
	_, err := repo.ApproveRequest(ctx, aprovacao("r2", "h2", instanteOAuth.Add(time.Minute)))
	require.NoError(t, err)

	_, err = repo.RedeemCode(ctx, "h1", instanteOAuth.Add(90*time.Second), janelaReusoCodigo)

	require.ErrorIs(t, err, oauth.ErrAuthorizationNotFound, "o código antigo não serve à Autorização nova")
}

func TestPostgresOAuthRepo_RedeemCode_ReusoMarcaOPedidoUmaVez(t *testing.T) {
	repo, pool := repoComCodigoDeA1(t)
	trocarCodigo(t, repo)
	primeiroReuso := instanteOAuth.Add(100 * time.Second)

	_, err := reusarCodigo(repo, primeiroReuso)
	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
	_, err = reusarCodigo(repo, primeiroReuso.Add(time.Minute))
	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)

	assert.Equal(t, 1, countRows(t, pool, `SELECT count(*) FROM oauth_requests
		WHERE id = 'r1' AND code_reused_at = '2026-10-16T12:01:40Z'`), "guarda o primeiro reuso")
}

func TestPostgresOAuthRepo_RedeemCode_ReusoRevogaSoOsTokensDaAutorizacaoDoPedido(t *testing.T) {
	repo, pool := repoComCodigoDeA1(t)
	trocarCodigo(t, repo)
	gravarTokens(t, pool, tokensOAuth("a1"))
	require.NoError(t, emitirTokenDe(pool, "a0", "t-outro"))

	got, err := reusarCodigo(repo, instanteOAuth.Add(100*time.Second))

	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
	assert.Equal(t, "a1", got.Authorization.ID)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE authorization_id = 'a1' AND NOT revoked"))
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE authorization_id = 'a0' AND NOT revoked"))
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations WHERE id = 'a1' AND revoked_at IS NULL"),
		"o reuso derruba os tokens, não a Autorização")
}

// TestPostgresOAuthRepo_RedeemCode_ReusoEsperaGravacaoDoPar prova que o reuso trava a
// Autorização: com a gravação do par segurando a trava, o reuso espera e, depois do
// commit, derruba também o par novo.
func TestPostgresOAuthRepo_RedeemCode_ReusoEsperaGravacaoDoPar(t *testing.T) {
	repo, pool := repoComCodigoDeA1(t)
	trocarCodigo(t, repo)
	gravacao := travarAutorizacao(t, pool, gravarParNovoNaTransacao)
	reusou := make(chan error, 1)
	go func() { _, err := reusarCodigo(repo, instanteOAuth.Add(100*time.Second)); reusou <- err }()
	esperarBloqueio(t, pool, "o reuso do código deveria esperar a trava da gravação do par")

	require.NoError(t, gravacao.Commit(context.Background()))

	require.ErrorIs(t, <-reusou, oauth.ErrCodeAlreadyUsed)
	assert.Equal(t, 0, tokensAtivos(t, pool))
}

func TestPostgresOAuthRepo_SaveCodeTokens_DepoisDoReuso_NaoGrava(t *testing.T) {
	repo, pool := repoComCodigoDeA1(t)
	redeemed := trocarCodigo(t, repo)
	_, err := reusarCodigo(repo, instanteOAuth.Add(100*time.Second))
	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)

	err = repo.SaveCodeTokens(context.Background(), redeemed.Request.ID, tokensOAuth(redeemed.Authorization.ID))

	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens"))
}

func TestPostgresOAuthRepo_SaveCodeTokens_DepoisDaRevogacaoPelaConta_NaoGrava(t *testing.T) {
	repo, pool := repoComCodigoDeA1(t)
	redeemed := trocarCodigo(t, repo)
	evento := eventoOAuth(oauth.AuditActionRevoke, oauth.AuditResultSuccess)
	require.NoError(t, repo.RevokeAuthorization(context.Background(), "u1", "a1", evento))

	err := repo.SaveCodeTokens(context.Background(), redeemed.Request.ID, tokensOAuth(redeemed.Authorization.ID))

	require.ErrorIs(t, err, oauth.ErrAuthorizationNotFound)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens"))
}

// TestPostgresOAuthRepo_SaveCodeTokens_EsperaQuemSeguraATrava cobre o que entra entre o
// consumo do código e a gravação do par: com o reuso do código ou a revogação pela conta
// segurando a trava, a gravação espera e, depois do commit, falha sem gravar.
func TestPostgresOAuthRepo_SaveCodeTokens_EsperaQuemSeguraATrava(t *testing.T) {
	casos := map[string]struct {
		comandos string
		erro     error
	}{
		"reuso do codigo": {"UPDATE oauth_requests SET code_reused_at = now() WHERE id = 'r1'", oauth.ErrCodeAlreadyUsed},
		"revogacao pela conta": {"UPDATE oauth_authorizations SET revoked_at = now() WHERE id = 'a1'",
			oauth.ErrAuthorizationNotFound},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			repo, pool := repoComCodigoDeA1(t)
			trocarCodigo(t, repo)
			concorrente := travarAutorizacao(t, pool, caso.comandos)
			gravou := make(chan error, 1)
			go func() { gravou <- repo.SaveCodeTokens(context.Background(), "r1", tokensOAuth("a1")) }()
			esperarBloqueio(t, pool, "a gravação do par deveria esperar a trava de "+nome)

			require.NoError(t, concorrente.Commit(context.Background()))

			require.ErrorIs(t, <-gravou, caso.erro)
			assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_tokens"))
		})
	}
}

// tokensDe renomeia os hashes de tokensOAuth com o sufixo, para dois pares conviverem.
func tokensDe(authorizationID, sufixo string) []oauth.IssuedToken {
	tokens := tokensOAuth(authorizationID)
	for i := range tokens {
		tokens[i].Hash += sufixo
	}
	return tokens
}

// trocarEGravar aprova o pedido, troca o código e grava o par dele, devolvendo a troca.
func trocarEGravar(t *testing.T, repo *storage.PostgresOAuthRepository, requestID, codeHash string, at time.Time) oauth.RedeemedCode {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.CreateRequest(ctx, pedidoOAuth(requestID, oauth.ScopeNotesRead)))
	_, err := repo.ApproveRequest(ctx, aprovacao(requestID, codeHash, at))
	require.NoError(t, err)
	redeemed, err := repo.RedeemCode(ctx, codeHash, at.Add(30*time.Second), janelaReusoCodigo)
	require.NoError(t, err)
	require.NoError(t, repo.SaveCodeTokens(ctx, requestID, tokensDe(redeemed.Authorization.ID, "-"+requestID)))
	return redeemed
}

func TestPostgresOAuthRepo_RedeemCode_ReusoAntigoDepoisDeReconsentir_NaoTocaANova(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	antiga := trocarEGravar(t, repo, "r1", "h1", instanteOAuth)
	evento := eventoOAuth(oauth.AuditActionRevoke, oauth.AuditResultSuccess)
	require.NoError(t, repo.RevokeAuthorization(ctx, "u1", antiga.Authorization.ID, evento))
	nova := trocarEGravar(t, repo, "r2", "h2", instanteOAuth.Add(2*time.Minute))

	got, err := repo.RedeemCode(ctx, "h1", instanteOAuth.Add(4*time.Minute), janelaReusoCodigo)

	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
	assert.Equal(t, antiga.Authorization.ID, got.Authorization.ID)
	assert.True(t, got.Authorization.Revoked)
	assert.Equal(t, 2, countRows(t, pool, "SELECT count(*) FROM oauth_tokens WHERE authorization_id = '"+nova.Authorization.ID+"' AND NOT revoked"),
		"os tokens da Autorização nova seguem vivos")
	_, err = repo.ActiveAuthorization(ctx, "u1", "c1")
	require.NoError(t, err, "a Autorização nova segue ativa")
}

func TestPostgresOAuthRepo_RedeemCode_ReusoAuditaComContaECliente(t *testing.T) {
	repo, pool := repoComCodigoDeA1(t)
	trocarCodigo(t, repo)

	_, err := reusarCodigo(repo, instanteOAuth.Add(100*time.Second))

	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
	assert.Equal(t, 1, countRows(t, pool, `SELECT count(*) FROM audit_events WHERE action = 'oauth.code_reuse'
		AND result = 'failure' AND actor_id = 'u1' AND target = 'c1' AND tenant_id IS NULL
		AND occurred_at = '2026-10-16T12:01:40Z'`))
}

// decisaoDeR1 é quando repoComCodigoDeA1 decide o pedido r1.
var decisaoDeR1 = instanteOAuth.Add(time.Minute)

func reusosDoCodigoAuditados(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	return countRows(t, pool, "SELECT count(*) FROM audit_events WHERE action = 'oauth.code_reuse'")
}

func TestPostgresOAuthRepo_RedeemCode_SegundoReuso_NaoRevogaNemAudita(t *testing.T) {
	repo, pool := repoComCodigoDeA1(t)
	trocarCodigo(t, repo)
	_, err := reusarCodigo(repo, instanteOAuth.Add(100*time.Second))
	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
	gravarTokens(t, pool, tokensDe("a1", "-novo"))

	_, err = reusarCodigo(repo, instanteOAuth.Add(160*time.Second))

	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
	assert.Equal(t, 2, tokensAtivos(t, pool), "só o primeiro reuso revoga")
	assert.Equal(t, 1, reusosDoCodigoAuditados(t, pool), "só o primeiro reuso audita")
}

func TestPostgresOAuthRepo_RedeemCode_ReusoForaDaJanela_NaoRevogaNemAudita(t *testing.T) {
	casos := map[string]time.Time{
		"um segundo depois da janela": decisaoDeR1.Add(janelaReusoCodigo + time.Second),
		"meses depois":                decisaoDeR1.AddDate(0, 6, 0),
	}
	for nome, instante := range casos {
		t.Run(nome, func(t *testing.T) {
			repo, pool := repoComCodigoDeA1(t)
			trocarCodigo(t, repo)
			gravarTokens(t, pool, tokensOAuth("a1"))

			got, err := reusarCodigo(repo, instante)

			require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
			assert.Equal(t, "r1", got.Request.ID)
			assert.Equal(t, 2, tokensAtivos(t, pool), "fora da janela o reuso não revoga")
			assert.Zero(t, reusosDoCodigoAuditados(t, pool), "fora da janela o reuso não audita")
			assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM oauth_requests WHERE id = 'r1' AND code_reused_at IS NULL"))
		})
	}
}

func TestPostgresOAuthRepo_RedeemCode_ReusoNoFimDaJanela_Revoga(t *testing.T) {
	repo, pool := repoComCodigoDeA1(t)
	trocarCodigo(t, repo)
	gravarTokens(t, pool, tokensOAuth("a1"))

	_, err := reusarCodigo(repo, decisaoDeR1.Add(janelaReusoCodigo))

	require.ErrorIs(t, err, oauth.ErrCodeAlreadyUsed)
	assert.Zero(t, tokensAtivos(t, pool))
	assert.Equal(t, 1, reusosDoCodigoAuditados(t, pool))
}
