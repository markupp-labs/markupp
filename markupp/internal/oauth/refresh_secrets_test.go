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

func TestRefreshToken_QualquerFalha_NaoVazaRefreshNemHash(t *testing.T) {
	casos := map[string]struct {
		findErr, consumeErr, saveErr, revokeErr error
		altera                                  func(*oauth.RefreshGrant)
	}{
		"client_id diferente":       {altera: func(g *oauth.RefreshGrant) { g.ClientID = "outro" }},
		"resource diferente":        {altera: func(g *oauth.RefreshGrant) { g.Resource = "https://outro.dev/mcp" }},
		"escopo alem do token":      {altera: func(g *oauth.RefreshGrant) { g.Scope = oauth.ScopeAdmin }},
		"refresh inexistente":       {consumeErr: oauth.ErrTokenNotFound},
		"autorizacao revogada":      {consumeErr: oauth.ErrAuthorizationNotFound},
		"refresh reusado":           {consumeErr: oauth.ErrRefreshReused},
		"falha ao ler":              {findErr: errors.New("banco fora do ar")},
		"falha ao consumir":         {consumeErr: errors.New("banco fora do ar")},
		"falha ao revogar no reuso": {consumeErr: oauth.ErrRefreshReused, revokeErr: errors.New("banco fora do ar")},
		"falha ao salvar":           {saveErr: errors.New("banco fora do ar")},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithRefreshableToken()
			repo.findErr, repo.consumeErr, repo.saveRefreshedErr, repo.revokeAuditedErr = caso.findErr, caso.consumeErr, caso.saveErr, caso.revokeErr
			grant := validRefreshGrant()
			if caso.altera != nil {
				caso.altera(&grant)
			}

			_, err := newTestService(repo).RefreshToken(context.Background(), grant)

			require.Error(t, err)
			assertSecretsAbsent(t, err, testRefreshToken, auth.HashOpaqueToken(testRefreshToken))
		})
	}
}

func TestRefreshToken_RefreshValido_RepositorioSoRecebeHashes(t *testing.T) {
	repo := testRepositoryWithRefreshableToken()

	response, err := newTestService(repo).RefreshToken(context.Background(), validRefreshGrant())

	require.NoError(t, err)
	secrets := []string{testRefreshToken, response.AccessToken, response.RefreshToken}
	assert.Len(t, map[string]bool{secrets[0]: true, secrets[1]: true, secrets[2]: true}, 3, "refresh antigo, access e refresh novos devem ser distintos")
	recorded := fmt.Sprintf("%+v %+v %+v", repo.consumeCalls, repo.refreshed, repo.refreshEvents)
	for _, secret := range secrets {
		assert.NotContains(t, recorded, secret, "token em claro chegou ao repositório")
	}
	require.Len(t, repo.refreshed, 1)
	assert.Equal(t, auth.HashOpaqueToken(response.AccessToken), repo.refreshed[0][0].Hash)
	assert.Equal(t, auth.HashOpaqueToken(response.RefreshToken), repo.refreshed[0][1].Hash)
}
