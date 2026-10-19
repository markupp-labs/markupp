package oauth_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

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

func TestRevokeToken_TokenDoCliente_DerrubaOsTokensDaAutorizacaoEAudita(t *testing.T) {
	casos := map[string]string{
		"com client_id do dono": testClientID,
		"sem client_id":         "",
	}
	for nome, clientID := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithFoundToken()

			err := newTestService(repo).RevokeToken(context.Background(), oauth.RevokeRequest{Token: testAccessToken, ClientID: clientID})

			require.NoError(t, err)
			assert.Equal(t, []string{auth.HashOpaqueToken(testAccessToken)}, repo.findCalls)
			assert.Equal(t, []auditedRevoke{{
				AuthorizationID: "aut-1", Event: testAuditEvent(oauth.AuditActionRevoke, oauth.AuditResultSuccess),
			}}, repo.auditedRevokes)
			assert.Empty(t, repo.revokedAuths, "o /oauth/revoke mantém o consentimento")
		})
	}
}

func TestRevokeToken_TokenVazio_InvalidRequestSemBuscar(t *testing.T) {
	repo := testRepositoryWithFoundToken()

	err := newTestService(repo).RevokeToken(context.Background(), oauth.RevokeRequest{ClientID: testClientID})

	requireProtocolError(t, err, "invalid_request", "", "")
	assert.Empty(t, repo.findCalls)
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

			err := newTestService(repo).RevokeToken(context.Background(), oauth.RevokeRequest{Token: testAccessToken, ClientID: testClientID})

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

			err := newTestService(repo).RevokeToken(context.Background(), oauth.RevokeRequest{Token: testAccessToken})

			requireNotProtocolError(t, err, causa)
			assertSecretsAbsent(t, err, testAccessToken, auth.HashOpaqueToken(testAccessToken))
		})
	}
}

func TestRevokeToken_RepositorioSoRecebeOHash(t *testing.T) {
	repo := testRepositoryWithFoundToken()

	err := newTestService(repo).RevokeToken(context.Background(), oauth.RevokeRequest{Token: testAccessToken})

	require.NoError(t, err)
	assert.NotContains(t, fmt.Sprintf("%+v %+v", repo.findCalls, repo.auditedRevokes), testAccessToken)
}
