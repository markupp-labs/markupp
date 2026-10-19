package oauth_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

const testAccessToken = "access-em-claro-1"

func testRepositoryWithFoundToken() *fakeOAuthRepository {
	repo := newFakeOAuthRepository()
	repo.found = testTokenGrant()
	return repo
}

func validRevokeRequest() oauth.RevokeRequest {
	return oauth.RevokeRequest{Token: testAccessToken, ClientID: testClientID}
}

func TestRevokeToken_TokenDoCliente_DerrubaOsTokensDaAutorizacaoEAudita(t *testing.T) {
	repo := testRepositoryWithFoundToken()

	err := newTestService(repo).RevokeToken(context.Background(), validRevokeRequest())

	require.NoError(t, err)
	assert.Equal(t, []string{auth.HashOpaqueToken(testAccessToken)}, repo.findCalls)
	assert.Equal(t, []auditedRevoke{{
		AuthorizationID: "aut-1", TriggerHash: repo.found.Token.Hash, RequireUnexpired: true,
		Event: testAuditEvent(oauth.AuditActionRevoke, oauth.AuditResultSuccess),
	}}, repo.auditedRevokes)
	assert.Empty(t, repo.revokedAuths, "o /oauth/revoke mantém o consentimento")
}

func TestRevokeToken_ParametroAusente_InvalidRequestSemBuscar(t *testing.T) {
	casos := map[string]oauth.RevokeRequest{
		"token vazio":     {ClientID: testClientID},
		"client_id vazio": {Token: testAccessToken},
	}
	for nome, req := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithFoundToken()

			err := newTestService(repo).RevokeToken(context.Background(), req)

			requireProtocolError(t, err, "invalid_request", "", "")
			assert.Empty(t, repo.findCalls)
		})
	}
}

func TestRevokeToken_TokenQueJaNaoVale_SucessoSemRevogarNemAuditar(t *testing.T) {
	casos := map[string]func(*oauth.TokenGrant){
		"token revogado":       func(g *oauth.TokenGrant) { g.Token.Revoked = true },
		"token expirado agora": func(g *oauth.TokenGrant) { g.Token.ExpiresAt = testNow },
		"token expirado antes": func(g *oauth.TokenGrant) { g.Token.ExpiresAt = testNow.Add(-time.Hour) },
		"autorizacao revogada": func(g *oauth.TokenGrant) { g.Authorization.Revoked = true },
	}
	for nome, altera := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithFoundToken()
			altera(&repo.found)

			err := newTestService(repo).RevokeToken(context.Background(), validRevokeRequest())

			require.NoError(t, err)
			assert.Len(t, repo.findCalls, 1)
			assert.Empty(t, repo.auditedRevokes)
		})
	}
}

func TestRevokeToken_TokenDesconhecidoOuDeOutroCliente_SucessoSemRevogar(t *testing.T) {
	casos := map[string]func() *fakeOAuthRepository{
		"token desconhecido": newFakeOAuthRepository,
		"client_id diferente do dono": func() *fakeOAuthRepository {
			repo := testRepositoryWithFoundToken()
			repo.found.Authorization.ClientID = "https://outro.dev/oauth.json"
			return repo
		},
	}
	for nome, repoDoCaso := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := repoDoCaso()

			err := newTestService(repo).RevokeToken(context.Background(), validRevokeRequest())

			require.NoError(t, err)
			assert.Len(t, repo.findCalls, 1)
			assert.Empty(t, repo.auditedRevokes)
		})
	}
}

func TestRevokeToken_FalhaDoRepositorio_DevolveErroEmbrulhadoSemVazarToken(t *testing.T) {
	casos := map[string]func(*fakeOAuthRepository) error{
		"ao buscar": func(r *fakeOAuthRepository) error { r.findErr = errors.New("banco fora do ar"); return r.findErr },
		"ao revogar": func(r *fakeOAuthRepository) error {
			r.revokeAuditedErr = errors.New("banco fora do ar")
			return r.revokeAuditedErr
		},
	}
	for nome, falha := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithFoundToken()
			causa := falha(repo)

			err := newTestService(repo).RevokeToken(context.Background(), validRevokeRequest())

			requireNotProtocolError(t, err, causa)
			assertSecretsAbsent(t, err, testAccessToken, auth.HashOpaqueToken(testAccessToken))
		})
	}
}

func TestRevokeToken_RepositorioSoRecebeOHash(t *testing.T) {
	repo := testRepositoryWithFoundToken()

	err := newTestService(repo).RevokeToken(context.Background(), validRevokeRequest())

	require.NoError(t, err)
	assert.NotContains(t, fmt.Sprintf("%+v %+v", repo.findCalls, repo.auditedRevokes), testAccessToken)
}

func TestRevokeToken_TokenRevogadoDepoisDaLeitura_SemErro(t *testing.T) {
	repo := testRepositoryWithFoundToken()
	repo.revokeAuditedStale = true

	err := newTestService(repo).RevokeToken(context.Background(), validRevokeRequest())

	require.NoError(t, err, "outra revogação entrou entre a leitura e a ação; nada a revelar ao cliente")
	assert.Len(t, repo.auditedRevokes, 1)
}
