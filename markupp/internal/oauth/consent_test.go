package oauth_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

const testUserID = "conta-1"

func pendingTestRequest() oauth.AuthorizationRequest {
	return oauth.AuthorizationRequest{
		ID:            "pedido-9",
		ClientID:      testClientID,
		RedirectURI:   "https://cliente.exemplo.dev:8443/callback?origem=x",
		Scopes:        []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite},
		Resource:      testResource,
		State:         "estado-do-cliente",
		CodeChallenge: testChallenge(testVerifier),
		CreatedAt:     testNow,
		ExpiresAt:     testNow.Add(testServiceConfig().RequestTTL),
	}
}

func testRepositoryWithPendingRequest() *fakeOAuthRepository {
	repo := testRepositoryWithClient()
	repo.requests["pedido-9"] = pendingTestRequest()
	return repo
}

func TestDescribeRequest_PedidoPendente_DevolveVisaoDoConsentimento(t *testing.T) {
	repo := testRepositoryWithPendingRequest()

	view, err := newTestService(repo).DescribeRequest(context.Background(), "pedido-9", testUserID)

	require.NoError(t, err)
	assert.Equal(t, oauth.RequestView{
		ID:           "pedido-9",
		ClientID:     testClientID,
		ClientName:   "Cliente de teste",
		ClientSource: oauth.ClientSourceCIMD,
		RedirectHost: "cliente.exemplo.dev:8443",
		Scopes:       []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite},
		ExpiresAt:    "2026-10-16T15:10:00Z",
	}, view)
	assert.Equal(t, []time.Time{testNow}, repo.pendingNow)
}

func TestDescribeRequest_RedirectComPorta_RedirectHostMantemAPorta(t *testing.T) {
	repo := testRepositoryWithPendingRequest()
	req := pendingTestRequest()
	req.RedirectURI = "http://127.0.0.1:53682/callback"
	repo.requests["pedido-9"] = req

	view, err := newTestService(repo).DescribeRequest(context.Background(), "pedido-9", testUserID)

	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:53682", view.RedirectHost)
}

func TestDescribeRequest_RedirectDeEsquemaPrivadoSemHost_RedirectHostMostraOEsquema(t *testing.T) {
	casos := map[string]string{
		"caminho absoluto": "cursor:/oauth/callback",
		"opaca":            "cursor:callback",
	}
	for nome, redirectURI := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithPendingRequest()
			req := pendingTestRequest()
			req.RedirectURI = redirectURI
			repo.requests["pedido-9"] = req

			view, err := newTestService(repo).DescribeRequest(context.Background(), "pedido-9", testUserID)

			require.NoError(t, err)
			assert.Equal(t, "cursor:", view.RedirectHost)
		})
	}
}

func TestDescribeRequest_ClienteDCR_MarcaAOrigemDoNome(t *testing.T) {
	repo := testRepositoryWithPendingRequest()
	client := testClient()
	client.Source = oauth.ClientSourceDCR
	repo.clients[testClientID] = client

	view, err := newTestService(repo).DescribeRequest(context.Background(), "pedido-9", testUserID)

	require.NoError(t, err)
	assert.Equal(t, oauth.ClientSourceDCR, view.ClientSource)
}

func TestDescribeRequest_AutorizacaoAtiva_MarcaJaAutorizadoSeCobreOsEscopos(t *testing.T) {
	casos := map[string]struct {
		concedidos []string
		esperado   bool
	}{
		"cobre exatamente": {[]string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite}, true},
		"cobre com sobra":  {oauth.SupportedScopes(), true},
		"cobre em parte":   {[]string{oauth.ScopeNotesRead}, false},
		"nao cobre nada":   {[]string{oauth.ScopeAdmin}, false},
		"sem escopos":      {nil, false},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithPendingRequest()
			repo.authorizations[authorizationKey(testUserID, testClientID)] = oauth.Authorization{ID: "aut-1", Scopes: caso.concedidos}

			view, err := newTestService(repo).DescribeRequest(context.Background(), "pedido-9", testUserID)

			require.NoError(t, err)
			assert.Equal(t, caso.esperado, view.AlreadyAuthorized)
		})
	}
}

func TestDescribeRequest_AutorizacaoDeOutraConta_NaoMarcaJaAutorizado(t *testing.T) {
	repo := testRepositoryWithPendingRequest()
	repo.authorizations[authorizationKey("outra-conta", testClientID)] = oauth.Authorization{ID: "aut-1", Scopes: oauth.SupportedScopes()}

	view, err := newTestService(repo).DescribeRequest(context.Background(), "pedido-9", testUserID)

	require.NoError(t, err)
	assert.False(t, view.AlreadyAuthorized)
}

func TestDescribeRequest_PedidoInexistente_DevolveErrRequestNotFound(t *testing.T) {
	_, err := newTestService(testRepositoryWithClient()).DescribeRequest(context.Background(), "pedido-x", testUserID)

	assert.ErrorIs(t, err, oauth.ErrRequestNotFound)
}

func TestDescribeRequest_ClienteApagadoDepoisDoPedido_DevolveErrRequestNotFound(t *testing.T) {
	repo := testRepositoryWithPendingRequest()
	repo.getClientErr = fmt.Errorf("%w: client_id %q", oauth.ErrClientNotFound, testClientID)

	_, err := newTestService(repo).DescribeRequest(context.Background(), "pedido-9", testUserID)

	assert.ErrorIs(t, err, oauth.ErrRequestNotFound)
}

func TestDescribeRequest_FalhaAoBuscarCliente_DevolveErroEmbrulhado(t *testing.T) {
	repo := testRepositoryWithPendingRequest()
	repo.getClientErr = errors.New("banco fora do ar")

	_, err := newTestService(repo).DescribeRequest(context.Background(), "pedido-9", testUserID)

	requireNotProtocolError(t, err, repo.getClientErr)
	assert.NotErrorIs(t, err, oauth.ErrRequestNotFound)
}

func TestDescribeRequest_FalhaNaAutorizacao_DevolveErroEmbrulhado(t *testing.T) {
	repo := testRepositoryWithPendingRequest()
	repo.activeErr = errors.New("banco fora do ar")

	_, err := newTestService(repo).DescribeRequest(context.Background(), "pedido-9", testUserID)

	requireNotProtocolError(t, err, repo.activeErr)
}

func TestApprove_PedidoPendente_GravaHashDoCodigoEDevolveRedirect(t *testing.T) {
	repo := testRepositoryWithPendingRequest()

	redirectTo, err := newTestService(repo).Approve(context.Background(), "pedido-9", testUserID)

	require.NoError(t, err)
	assert.Equal(t, []oauth.Approval{{
		RequestID:     "pedido-9",
		UserID:        testUserID,
		CodeHash:      auth.HashOpaqueToken("token-1"),
		CodeExpiresAt: testNow.Add(testServiceConfig().CodeTTL),
		DecidedAt:     testNow,

		DCRPendingCutoff: testNow.Add(-testDCRPendingTTL),
	}}, repo.approvals)
	assertRedirect(t, redirectTo, url.Values{
		"origem": {"x"}, "code": {"token-1"}, "state": {"estado-do-cliente"}, "iss": {testIssuer},
	})
}

func TestApprove_PedidoInexistente_DevolveErrRequestNotFound(t *testing.T) {
	_, err := newTestService(testRepositoryWithClient()).Approve(context.Background(), "pedido-x", testUserID)

	assert.ErrorIs(t, err, oauth.ErrRequestNotFound)
}

func TestApprove_FalhaDoRepositorio_DevolveErroEmbrulhado(t *testing.T) {
	repo := testRepositoryWithPendingRequest()
	repo.approveErr = errors.New("banco fora do ar")

	_, err := newTestService(repo).Approve(context.Background(), "pedido-9", testUserID)

	requireNotProtocolError(t, err, repo.approveErr)
}

func TestDeny_PedidoPendente_RegistraRecusaEDevolveAccessDenied(t *testing.T) {
	repo := testRepositoryWithPendingRequest()

	redirectTo, err := newTestService(repo).Deny(context.Background(), "pedido-9", testUserID)

	require.NoError(t, err)
	assert.Equal(t, []oauth.Denial{{RequestID: "pedido-9", UserID: testUserID, DecidedAt: testNow}}, repo.denials)
	assertRedirect(t, redirectTo, url.Values{
		"origem": {"x"}, "error": {"access_denied"}, "state": {"estado-do-cliente"}, "iss": {testIssuer},
	})
}

func TestDeny_PedidoInexistente_DevolveErrRequestNotFound(t *testing.T) {
	_, err := newTestService(testRepositoryWithClient()).Deny(context.Background(), "pedido-x", testUserID)

	assert.ErrorIs(t, err, oauth.ErrRequestNotFound)
}

// assertRedirect confere que o redirect volta à redirect_uri do pedido com a query esperada.
func assertRedirect(t *testing.T, redirectTo string, query url.Values) {
	t.Helper()
	parsed, err := url.Parse(redirectTo)
	require.NoError(t, err)
	assert.Equal(t, "https://cliente.exemplo.dev:8443/callback", parsed.Scheme+"://"+parsed.Host+parsed.Path)
	assert.Equal(t, query, parsed.Query())
}
