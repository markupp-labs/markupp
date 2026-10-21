package oauth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// O resource da API é o próprio issuer (ADR-0042): o cliente chama a API com o JWT curto
// e guarda só o refresh.

func TestStartAuthorization_ResourceDaAPI_Aceita(t *testing.T) {
	repo := testRepositoryWithClient()
	params := validAuthorizeParams()
	params.Resource = testIssuer + "/"

	_, err := newTestService(repo).StartAuthorization(context.Background(), params)

	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	assert.Equal(t, testIssuer, repo.created[0].Resource)
}

func repositoryWithAPICode() *fakeOAuthRepository {
	repo := testRepositoryWithRedeemableCode()
	repo.redeemed.Request.Resource = testIssuer
	return repo
}

func TestExchangeCode_ResourceDaAPI_EntregaOJWTEGravaSoORefresh(t *testing.T) {
	repo := repositoryWithAPICode()
	scoped := &fakeScopedTokenIssuer{}
	grant := validCodeGrant()
	grant.Resource = testIssuer

	response, err := newTestServiceWithIssuer(repo, scoped).ExchangeCode(context.Background(), grant)

	require.NoError(t, err)
	assert.Equal(t, oauth.TokenResponse{
		AccessToken: testScopedToken, TokenType: "Bearer", ExpiresIn: 300,
		RefreshToken: "token-1", Scope: "notas:leitura notas:escrita",
	}, response)
	scopes := []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite}
	assert.Equal(t, []scopedIssue{{UserID: testUserID, Scopes: scopes, TTL: 5 * time.Minute}}, scoped.issues)
	assert.Equal(t, [][]oauth.IssuedToken{{{
		Hash: auth.HashOpaqueToken("token-1"), AuthorizationID: "aut-1", Kind: oauth.TokenKindRefresh,
		Resource: testIssuer, Scopes: scopes, ExpiresAt: testNow.Add(30 * 24 * time.Hour),
	}}}, repo.saved, "o JWT nao e gravado; so o refresh")
}

func TestExchangeCode_ResourceDaAPIComFalhaNoJWT_NaoGrava(t *testing.T) {
	repo := repositoryWithAPICode()
	scoped := &fakeScopedTokenIssuer{err: assert.AnError}

	_, err := newTestServiceWithIssuer(repo, scoped).ExchangeCode(context.Background(), validCodeGrant())

	require.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, repo.saved)
}

func TestRefreshToken_ResourceDaAPI_EntregaNovoJWTEGiraORefresh(t *testing.T) {
	repo := testRepositoryWithRefreshableToken()
	repo.found.Token.Resource = testIssuer
	repo.consumed.Token.Resource = testIssuer
	scoped := &fakeScopedTokenIssuer{}

	response, err := newTestServiceWithIssuer(repo, scoped).RefreshToken(context.Background(), validRefreshGrant())

	require.NoError(t, err)
	assert.Equal(t, testScopedToken, response.AccessToken)
	assert.Equal(t, int64(300), response.ExpiresIn)
	assert.Equal(t, "token-1", response.RefreshToken)
	require.Len(t, repo.refreshed, 1)
	require.Len(t, repo.refreshed[0], 1, "so o refresh novo e gravado")
	assert.Equal(t, oauth.TokenKindRefresh, repo.refreshed[0][0].Kind)
	assert.Len(t, scoped.issues, 1)
}
