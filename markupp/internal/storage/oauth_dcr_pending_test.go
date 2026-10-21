package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func TestPostgresOAuthRepo_GetClient_PendenteVencidoContaComoInexistente(t *testing.T) {
	repo, pool := repoDCR(t)
	clientesEmVoltaDoLimite(t, pool)
	limite := instanteOAuth.Add(-prazoDosPendentes)

	for _, id := range []string{"dcr-vencido", "dcr-revogado-vencido"} {
		_, err := repo.GetClient(context.Background(), id, limite)
		require.ErrorIs(t, err, oauth.ErrClientNotFound, id)
	}
	for _, id := range []string{"dcr-quase", "dcr-autorizado", "dcr-revogado-quase", "cimd-antigo"} {
		_, err := repo.GetClient(context.Background(), id, limite)
		assert.NoError(t, err, id)
	}
}

func TestPostgresOAuthRepo_ApproveRequest_ClientePendenteVencido_RecusaSemGravar(t *testing.T) {
	repo, pool := repoDCR(t)
	ctx := context.Background()
	clientesEmVoltaDoLimite(t, pool)
	pedido := pedidoOAuth("r1", oauth.ScopeNotesRead)
	pedido.ClientID = "dcr-vencido"
	require.NoError(t, repo.CreateRequest(ctx, pedido))
	aprovar := aprovacao("r1", "h1", instanteOAuth)
	aprovar.DCRPendingCutoff = instanteOAuth.Add(-prazoDosPendentes)

	_, err := repo.ApproveRequest(ctx, aprovar)

	require.ErrorIs(t, err, oauth.ErrRequestNotFound)
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations WHERE client_id = 'dcr-vencido'"))
	_, err = repo.GetPendingRequest(ctx, "r1", instanteOAuth)
	require.NoError(t, err, "o pedido continua pendente")
}

func TestPostgresOAuthRepo_ApproveRequest_ClientePendenteNoPrazo_Aprova(t *testing.T) {
	repo, pool := repoDCR(t)
	ctx := context.Background()
	clientesEmVoltaDoLimite(t, pool)
	pedido := pedidoOAuth("r1", oauth.ScopeNotesRead)
	pedido.ClientID = "dcr-quase"
	require.NoError(t, repo.CreateRequest(ctx, pedido))
	aprovar := aprovacao("r1", "h1", instanteOAuth)
	aprovar.DCRPendingCutoff = instanteOAuth.Add(-prazoDosPendentes)

	_, err := repo.ApproveRequest(ctx, aprovar)

	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations WHERE client_id = 'dcr-quase'"))
}
