package oauth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func testAuthorizationSummary() oauth.AuthorizationSummary {
	authorization := testAuthorization()
	authorization.CreatedAt = testNow
	authorization.UpdatedAt = testNow.Add(90 * time.Minute)
	return oauth.AuthorizationSummary{Authorization: authorization, ClientName: "Cliente de teste"}
}

func testRepositoryWithActiveAuthorization() *fakeOAuthRepository {
	repo := newFakeOAuthRepository()
	repo.summaries = []oauth.AuthorizationSummary{testAuthorizationSummary()}
	return repo
}

func TestListAuthorizations_AutorizacoesAtivas_DevolveVisaoComDatasUTC(t *testing.T) {
	repo := testRepositoryWithActiveAuthorization()

	views, err := newTestService(repo).ListAuthorizations(context.Background(), testUserID)

	require.NoError(t, err)
	assert.Equal(t, []oauth.AuthorizationView{{
		ID: "aut-1", ClientID: testClientID, ClientName: "Cliente de teste", Scopes: oauth.SupportedScopes(),
		CreatedAt: "2026-10-16T15:00:00Z", UpdatedAt: "2026-10-16T16:30:00Z",
	}}, views)
	assert.Equal(t, []string{testUserID}, repo.listCalls)
}

func TestListAuthorizations_SemAutorizacoes_SliceVazioNaoNil(t *testing.T) {
	casos := map[string][]oauth.AuthorizationSummary{
		"nil do repositorio":   nil,
		"vazia do repositorio": {},
	}
	for nome, summaries := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := newFakeOAuthRepository()
			repo.summaries = summaries

			views, err := newTestService(repo).ListAuthorizations(context.Background(), testUserID)

			require.NoError(t, err)
			assert.NotNil(t, views)
			assert.Empty(t, views)
		})
	}
}

func TestListAuthorizations_FalhaDoRepositorio_DevolveErroEmbrulhado(t *testing.T) {
	repo := newFakeOAuthRepository()
	repo.listErr = errors.New("banco fora do ar")

	_, err := newTestService(repo).ListAuthorizations(context.Background(), testUserID)

	require.ErrorIs(t, err, repo.listErr)
}

// testRevokeAuthorizationEvent é o evento que o serviço manda; o cliente alvo o
// repositório preenche pela Autorização, na transação.
func testRevokeAuthorizationEvent() oauth.AuditEvent {
	event := testAuditEvent(oauth.AuditActionRevoke, oauth.AuditResultSuccess)
	event.ClientID = ""
	return event
}

func TestRevokeAuthorization_AutorizacaoDaConta_RevogaEAuditaSemLerAntes(t *testing.T) {
	repo := newFakeOAuthRepository()

	err := newTestService(repo).RevokeAuthorization(context.Background(), testUserID, "aut-1")

	require.NoError(t, err)
	assert.Equal(t, []revokedAuthorization{{
		UserID: testUserID, AuthorizationID: "aut-1", Event: testRevokeAuthorizationEvent(),
	}}, repo.revokedAuths)
	assert.Empty(t, repo.listCalls, "o cliente alvo vem da transação do repositório")
	assert.Empty(t, repo.auditedRevokes)
}

func TestRevokeAuthorization_AutorizacaoInexistente_ErrAuthorizationNotFound(t *testing.T) {
	casos := map[string]error{
		"inexistente, revogada ou de outra conta": oauth.ErrAuthorizationNotFound,
		"embrulhado": errors.Join(errors.New("contexto"), oauth.ErrAuthorizationNotFound),
	}
	for nome, causa := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := newFakeOAuthRepository()
			repo.revokeAuthErr = causa

			err := newTestService(repo).RevokeAuthorization(context.Background(), testUserID, "aut-1")

			require.ErrorIs(t, err, oauth.ErrAuthorizationNotFound)
			assert.Len(t, repo.revokedAuths, 1)
		})
	}
}

func TestRevokeAuthorization_FalhaDoRepositorio_DevolveErroEmbrulhado(t *testing.T) {
	repo := newFakeOAuthRepository()
	repo.revokeAuthErr = errors.New("banco fora do ar")

	err := newTestService(repo).RevokeAuthorization(context.Background(), testUserID, "aut-1")

	require.ErrorIs(t, err, repo.revokeAuthErr)
	assert.NotErrorIs(t, err, oauth.ErrAuthorizationNotFound)
}
