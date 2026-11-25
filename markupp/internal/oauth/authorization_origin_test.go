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

var origemDoAgente = auth.Origin{IP: "198.51.100.9", UserAgent: "Claude-User/1.0"}

func comOrigemDoAgente() context.Context {
	return auth.WithOrigin(context.Background(), origemDoAgente)
}

func toqueEsperado() []authorizationTouch {
	return []authorizationTouch{{AuthorizationID: "aut-1", Origin: origemDoAgente, At: testNow}}
}

func TestExchangeCode_RegistraOrigemEUltimoUsoDaAutorizacao(t *testing.T) {
	repo := testRepositoryWithRedeemableCode()

	_, err := newTestService(repo).ExchangeCode(comOrigemDoAgente(), validCodeGrant())

	require.NoError(t, err)
	assert.Equal(t, toqueEsperado(), repo.touches)
}

func TestRefreshToken_RegistraOrigemEUltimoUsoDaAutorizacao(t *testing.T) {
	repo := testRepositoryWithRefreshableToken()

	_, err := newTestService(repo).RefreshToken(comOrigemDoAgente(), validRefreshGrant())

	require.NoError(t, err)
	assert.Equal(t, toqueEsperado(), repo.touches)
}

func TestExchangeToken_RegistraOrigemEUltimoUsoDaAutorizacao(t *testing.T) {
	repo, issuer := testRepositoryWithAccessToken(), &fakeScopedTokenIssuer{}

	_, err := newTestServiceWithIssuer(repo, issuer).ExchangeToken(comOrigemDoAgente(), validExchangeGrant())

	require.NoError(t, err)
	assert.Equal(t, toqueEsperado(), repo.touches)
}

func TestExchangeToken_Recusado_NaoRegistraUso(t *testing.T) {
	grant := validExchangeGrant()
	grant.SubjectToken = "token-desconhecido"
	repo, _, _, err := exchange(t, grant, func(r *fakeOAuthRepository) { r.findErr = oauth.ErrTokenNotFound })

	require.Error(t, err)
	assert.Empty(t, repo.touches)
}

func TestListAuthorizations_MostraOrigemEUltimoUso(t *testing.T) {
	repo := testRepositoryWithActiveAuthorization()
	repo.summaries[0].Origin = origemDoAgente
	repo.summaries[0].LastUsedAt = testNow.Add(2 * time.Hour)

	views, err := newTestService(repo).ListAuthorizations(context.Background(), testUserID)

	require.NoError(t, err)
	assert.Equal(t, "198.51.100.9", views[0].IP)
	assert.Equal(t, "Claude-User/1.0", views[0].UserAgent)
	assert.Equal(t, "2026-10-16T17:00:00Z", views[0].LastUsedAt)
}

func TestListAuthorizations_NuncaUsada_SemUltimoUso(t *testing.T) {
	views, err := newTestService(testRepositoryWithActiveAuthorization()).ListAuthorizations(context.Background(), testUserID)

	require.NoError(t, err)
	assert.Empty(t, views[0].LastUsedAt)
}
