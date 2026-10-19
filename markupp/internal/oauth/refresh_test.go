package oauth_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

const testRefreshToken = "refresh-antigo-1"

func testAuthorization() oauth.Authorization {
	return oauth.Authorization{ID: "aut-1", UserID: testUserID, ClientID: testClientID, Scopes: oauth.SupportedScopes()}
}

// testTokenGrant é o refresh token vigente da aut-1, com leitura e escrita.
func testTokenGrant() oauth.TokenGrant {
	return oauth.TokenGrant{
		Token: oauth.IssuedToken{
			Hash: auth.HashOpaqueToken(testRefreshToken), AuthorizationID: "aut-1", Kind: oauth.TokenKindRefresh,
			Resource: testResource, Scopes: []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite},
			ExpiresAt: testNow.Add(time.Hour),
		},
		Authorization: testAuthorization(),
	}
}

func testRepositoryWithRefreshableToken() *fakeOAuthRepository {
	repo := newFakeOAuthRepository()
	repo.consumed = testTokenGrant()
	return repo
}

func validRefreshGrant() oauth.RefreshGrant {
	return oauth.RefreshGrant{RefreshToken: testRefreshToken, ClientID: testClientID}
}

func testAuditEvent(action, result string) oauth.AuditEvent {
	return oauth.AuditEvent{Action: action, Result: result, UserID: testUserID, ClientID: testClientID, OccurredAt: testNow}
}

func TestRefreshToken_RefreshValido_RotacionaOParEAudita(t *testing.T) {
	repo := testRepositoryWithRefreshableToken()

	response, err := newTestService(repo).RefreshToken(context.Background(), validRefreshGrant())

	require.NoError(t, err)
	assert.Equal(t, oauth.TokenResponse{
		AccessToken: "token-1", TokenType: "Bearer", ExpiresIn: 3600,
		RefreshToken: "token-2", Scope: "notas:leitura notas:escrita",
	}, response)
	assert.Equal(t, []string{auth.HashOpaqueToken(testRefreshToken)}, repo.consumeCalls)
	assert.Equal(t, []time.Time{testNow}, repo.consumeNow)
	assert.Equal(t, []string{auth.HashOpaqueToken(testRefreshToken)}, repo.refreshParents, "a gravação confere o refresh pai")
	assert.Equal(t, [][]oauth.IssuedToken{expectedIssuedTokens()}, repo.refreshed)
	assert.Equal(t, []oauth.AuditEvent{testAuditEvent(oauth.AuditActionRefresh, oauth.AuditResultSuccess)}, repo.refreshEvents)
	assert.Empty(t, repo.saved, "o refresh grava pelo SaveRefreshedTokens, que audita")
}

func TestRefreshToken_Escopo_MantemOuEstreita(t *testing.T) {
	casos := map[string]struct {
		scope string
		want  []string
	}{
		"ausente mantem os do token":      {"", []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite}},
		"so espacos mantem os do token":   {"  ", []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite}},
		"subconjunto estreita":            {oauth.ScopeNotesRead, []string{oauth.ScopeNotesRead}},
		"todos repetidos e fora de ordem": {"notas:escrita notas:leitura notas:escrita", []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite}},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithRefreshableToken()
			grant := validRefreshGrant()
			grant.Scope = caso.scope

			response, err := newTestService(repo).RefreshToken(context.Background(), grant)

			require.NoError(t, err)
			require.Len(t, repo.refreshed, 1)
			assert.Equal(t, caso.want, repo.refreshed[0][0].Scopes)
			assert.Equal(t, caso.want, repo.refreshed[0][1].Scopes)
			assert.Equal(t, strings.Join(caso.want, " "), response.Scope)
		})
	}
}

func TestRefreshToken_ResourceEquivalente_MantemODoToken(t *testing.T) {
	repo := testRepositoryWithRefreshableToken()
	grant := validRefreshGrant()
	grant.Resource = "https://MARKUPP.exemplo.dev/mcp/"

	_, err := newTestService(repo).RefreshToken(context.Background(), grant)

	require.NoError(t, err)
	require.Len(t, repo.refreshed, 1)
	assert.Equal(t, testResource, repo.refreshed[0][0].Resource)
	assert.Equal(t, testResource, repo.refreshed[0][1].Resource)
}

func TestRefreshToken_ParametroAusente_InvalidRequestSemConsumir(t *testing.T) {
	casos := map[string]func(*oauth.RefreshGrant){
		"refresh_token vazio": func(g *oauth.RefreshGrant) { g.RefreshToken = "" },
		"client_id vazio":     func(g *oauth.RefreshGrant) { g.ClientID = "" },
	}
	for nome, altera := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithRefreshableToken()
			grant := validRefreshGrant()
			altera(&grant)

			_, err := newTestService(repo).RefreshToken(context.Background(), grant)

			requireProtocolError(t, err, "invalid_request", "", "")
			assert.Empty(t, repo.consumeCalls)
		})
	}
}

func TestRefreshToken_RefreshRecusadoPeloRepositorio_InvalidGrantSemRevogar(t *testing.T) {
	casos := map[string]error{
		"inexistente, expirado ou revogado": oauth.ErrTokenNotFound,
		"inexistente embrulhado":            errors.Join(errors.New("contexto"), oauth.ErrTokenNotFound),
		"autorizacao revogada":              oauth.ErrAuthorizationNotFound,
	}
	for nome, causa := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := newFakeOAuthRepository()
			repo.consumeErr = causa

			_, err := newTestService(repo).RefreshToken(context.Background(), validRefreshGrant())

			requireProtocolError(t, err, "invalid_grant", "", "")
			assert.Empty(t, repo.refreshed)
			assert.Empty(t, repo.auditedRevokes)
		})
	}
}

// TestRefreshToken_RevogadoAntesDeGravar_InvalidGrant cobre a revogação que entra entre
// o consumo e a gravação do par novo.
func TestRefreshToken_RevogadoAntesDeGravar_InvalidGrant(t *testing.T) {
	casos := map[string]error{
		"refresh pai revogado": oauth.ErrTokenNotFound,
		"autorizacao revogada": fmt.Errorf("contexto: %w", oauth.ErrAuthorizationNotFound),
	}
	for nome, causa := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithRefreshableToken()
			repo.saveRefreshedErr = causa

			_, err := newTestService(repo).RefreshToken(context.Background(), validRefreshGrant())

			requireProtocolError(t, err, "invalid_grant", "", "")
			assert.Empty(t, repo.auditedRevokes)
		})
	}
}

func TestRefreshToken_RefreshJaRotacionado_RevogaTokensEAuditaReuso(t *testing.T) {
	repo := testRepositoryWithRefreshableToken()
	repo.consumeErr = oauth.ErrRefreshReused

	_, err := newTestService(repo).RefreshToken(context.Background(), validRefreshGrant())

	requireProtocolError(t, err, "invalid_grant", "", "")
	assert.Equal(t, []auditedRevoke{{
		AuthorizationID: "aut-1", Event: testAuditEvent(oauth.AuditActionRefreshReuse, oauth.AuditResultFailure),
	}}, repo.auditedRevokes)
	assert.Empty(t, repo.refreshed)
	assert.Empty(t, repo.revokedAuths, "o reuso derruba os tokens, não a Autorização")
}

func TestRefreshToken_ReusoSemAutorizacao_NaoRevoga(t *testing.T) {
	repo := newFakeOAuthRepository()
	repo.consumeErr = oauth.ErrRefreshReused

	_, err := newTestService(repo).RefreshToken(context.Background(), validRefreshGrant())

	requireProtocolError(t, err, "invalid_grant", "", "")
	assert.Empty(t, repo.auditedRevokes)
}

func TestRefreshToken_ClientIDDiferente_InvalidGrantComRefreshConsumido(t *testing.T) {
	repo := testRepositoryWithRefreshableToken()
	grant := validRefreshGrant()
	grant.ClientID = "https://outro.dev/oauth.json"

	_, err := newTestService(repo).RefreshToken(context.Background(), grant)

	requireProtocolError(t, err, "invalid_grant", "", "")
	assert.Len(t, repo.consumeCalls, 1, "o refresh é consumido antes das conferências")
	assert.Empty(t, repo.refreshed)
	assert.Empty(t, repo.auditedRevokes)
}

func TestRefreshToken_ResourceQueNaoBateComOToken_InvalidTarget(t *testing.T) {
	casos := map[string]string{
		"resource diferente":      "https://outro.dev/mcp",
		"resource ilegivel":       "http://[::1",
		"resource com outro path": testResource + "/sub",
	}
	for nome, resource := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithRefreshableToken()
			grant := validRefreshGrant()
			grant.Resource = resource

			_, err := newTestService(repo).RefreshToken(context.Background(), grant)

			requireProtocolError(t, err, "invalid_target", "", "")
			assert.Len(t, repo.consumeCalls, 1, "o refresh é consumido antes das conferências")
			assert.Empty(t, repo.refreshed)
		})
	}
}

func TestRefreshToken_EscopoAlemDoToken_InvalidScope(t *testing.T) {
	casos := map[string]string{
		"escopo suportado nao concedido": oauth.ScopeAdmin,
		"escopo desconhecido":            "notas:tudo",
		"um a mais junto dos concedidos": "notas:leitura administracao",
	}
	for nome, scope := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithRefreshableToken()
			grant := validRefreshGrant()
			grant.Scope = scope

			_, err := newTestService(repo).RefreshToken(context.Background(), grant)

			requireProtocolError(t, err, "invalid_scope", "", "")
			assert.Empty(t, repo.refreshed)
		})
	}
}

func TestRefreshToken_FalhaDoRepositorio_DevolveErroEmbrulhado(t *testing.T) {
	casos := map[string]func(*fakeOAuthRepository) error{
		"ao consumir": func(r *fakeOAuthRepository) error { r.consumeErr = errors.New("banco fora do ar"); return r.consumeErr },
		"ao salvar": func(r *fakeOAuthRepository) error {
			r.saveRefreshedErr = errors.New("banco fora do ar")
			return r.saveRefreshedErr
		},
		"ao revogar apos reuso": func(r *fakeOAuthRepository) error {
			r.consumeErr = oauth.ErrRefreshReused
			r.revokeAuditedErr = errors.New("banco fora do ar")
			return r.revokeAuditedErr
		},
	}
	for nome, falha := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithRefreshableToken()
			causa := falha(repo)

			_, err := newTestService(repo).RefreshToken(context.Background(), validRefreshGrant())

			requireNotProtocolError(t, err, causa)
		})
	}
}
