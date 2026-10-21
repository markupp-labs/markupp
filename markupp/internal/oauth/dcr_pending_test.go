package oauth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// testDCRClientID é um cliente registrado por DCR, ainda sem Autorização.
const testDCRClientID = "markupp-dcr-pendente"

// testRepositoryWithPendingDCR grava o cliente DCR pendente criado em createdAt e um
// pedido pendente dele.
func testRepositoryWithPendingDCR(createdAt time.Time) *fakeOAuthRepository {
	repo := newFakeOAuthRepository()
	repo.clients[testDCRClientID] = oauth.Client{ID: testDCRClientID, Name: "Agente DCR", RedirectURIs: []string{testRedirectURI}}
	repo.pendingDCRCreatedAt[testDCRClientID] = createdAt
	req := pendingTestRequest()
	req.ClientID = testDCRClientID
	repo.requests[req.ID] = req
	return repo
}

func dcrAuthorizeParams() oauth.AuthorizeParams {
	params := validAuthorizeParams()
	params.ClientID = testDCRClientID
	return params
}

func TestStartAuthorization_ClienteDCRPendenteVencido_TratadoComoInexistente(t *testing.T) {
	repo := testRepositoryWithPendingDCR(testNow.Add(-testDCRPendingTTL))

	_, err := newTestService(repo).StartAuthorization(context.Background(), dcrAuthorizeParams())

	requireProtocolError(t, err, "invalid_request", "", "")
	assert.Equal(t, []time.Time{testNow.Add(-testDCRPendingTTL)}, repo.getClientCutoffs)
	assert.Empty(t, repo.created)
}

func TestStartAuthorization_ClienteDCRPendenteNoPrazo_Aceito(t *testing.T) {
	repo := testRepositoryWithPendingDCR(testNow.Add(-testDCRPendingTTL + time.Second))

	_, err := newTestService(repo).StartAuthorization(context.Background(), dcrAuthorizeParams())

	require.NoError(t, err)
	assert.Len(t, repo.created, 1)
}

func TestDescribeRequest_ClienteDCRPendenteVencido_DevolveErrRequestNotFound(t *testing.T) {
	repo := testRepositoryWithPendingDCR(testNow.Add(-testDCRPendingTTL))

	_, err := newTestService(repo).DescribeRequest(context.Background(), "pedido-9", testUserID)

	assert.ErrorIs(t, err, oauth.ErrRequestNotFound)
	assert.Equal(t, []time.Time{testNow.Add(-testDCRPendingTTL)}, repo.getClientCutoffs)
}

func TestApprove_ClienteDCRPendenteVencido_DevolveErrRequestNotFound(t *testing.T) {
	repo := testRepositoryWithPendingDCR(testNow.Add(-testDCRPendingTTL))

	_, err := newTestService(repo).Approve(context.Background(), "pedido-9", testUserID)

	assert.ErrorIs(t, err, oauth.ErrRequestNotFound)
	require.Len(t, repo.approvals, 1)
	assert.Equal(t, testNow.Add(-testDCRPendingTTL), repo.approvals[0].DCRPendingCutoff)
}
