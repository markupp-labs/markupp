package oauth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockedAccount_TrocaDoCodigo_InvalidGrant(t *testing.T) {
	repo := testRepositoryWithRedeemableCode()
	repo.blocked = map[string]bool{testUserID: true}

	_, err := newTestService(repo).ExchangeCode(context.Background(), validCodeGrant())

	requireProtocolError(t, err, "invalid_grant", "", "")
	assert.Empty(t, repo.saved, "conta bloqueada não ganha token")
}

func TestBlockedAccount_Refresh_InvalidGrant(t *testing.T) {
	repo := testRepositoryWithRefreshableToken()
	repo.blocked = map[string]bool{testUserID: true}

	_, err := newTestService(repo).RefreshToken(context.Background(), validRefreshGrant())

	requireProtocolError(t, err, "invalid_grant", "", "")
	assert.Empty(t, repo.refreshed)
}

func TestBlockedAccount_TrocaDoMCP_InvalidGrant(t *testing.T) {
	grant := validExchangeGrant()
	_, issuer, _, err := exchange(t, grant, func(r *fakeOAuthRepository) { r.blocked = map[string]bool{testUserID: true} })

	requireProtocolError(t, err, "invalid_grant", "", "")
	assert.Empty(t, issuer.issues)
}
