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

func TestExchangeCode_QualquerFalha_NaoVazaCodigoNemVerifier(t *testing.T) {
	casos := map[string]struct {
		repo   func() *fakeOAuthRepository
		altera func(*oauth.CodeGrant)
	}{
		"client_id diferente":    {testRepositoryWithRedeemableCode, func(g *oauth.CodeGrant) { g.ClientID = "outro" }},
		"redirect_uri diferente": {testRepositoryWithRedeemableCode, func(g *oauth.CodeGrant) { g.RedirectURI = testRedirectURI }},
		"verifier errado":        {testRepositoryWithRedeemableCode, func(g *oauth.CodeGrant) { g.CodeVerifier += "x" }},
		"resource diferente":     {testRepositoryWithRedeemableCode, func(g *oauth.CodeGrant) { g.Resource = "https://outro.dev/mcp" }},
		"codigo inexistente":     {repoFailingRedeem(oauth.ErrCodeNotFound, nil), nil},
		"sem autorizacao ativa":  {repoFailingRedeem(oauth.ErrAuthorizationNotFound, nil), nil},
		"codigo reusado":         {repoFailingRedeem(oauth.ErrCodeAlreadyUsed, nil), nil},
		"falha ao consumir":      {repoFailingRedeem(errors.New("banco fora do ar"), nil), nil},
		"falha ao revogar":       {repoFailingRedeem(oauth.ErrCodeAlreadyUsed, errors.New("banco fora do ar")), nil},
		"falha ao salvar":        {repoFailingSave, nil},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			grant := validCodeGrant()
			if caso.altera != nil {
				caso.altera(&grant)
			}

			_, err := newTestService(caso.repo()).ExchangeCode(context.Background(), grant)

			require.Error(t, err)
			assertSecretsAbsent(t, err, grant.Code, grant.CodeVerifier)
		})
	}
}

// repoFailingRedeem devolve o repositório cujo RedeemCode falha com redeemErr e cujo
// RevokeAuthorizationTokens falha com revokeErr.
func repoFailingRedeem(redeemErr, revokeErr error) func() *fakeOAuthRepository {
	return func() *fakeOAuthRepository {
		repo := testRepositoryWithRedeemableCode()
		repo.redeemErr, repo.revokeErr = redeemErr, revokeErr
		return repo
	}
}

func repoFailingSave() *fakeOAuthRepository {
	repo := testRepositoryWithRedeemableCode()
	repo.saveErr = errors.New("banco fora do ar")
	return repo
}

// assertSecretsAbsent confere que nenhum segredo aparece no erro nem na descrição.
func assertSecretsAbsent(t *testing.T, err error, secrets ...string) {
	t.Helper()
	description := ""
	var protocolErr *oauth.ProtocolError
	if errors.As(err, &protocolErr) {
		description = protocolErr.Description
	}
	for _, secret := range secrets {
		assert.NotContains(t, err.Error(), secret)
		assert.NotContains(t, description, secret)
	}
}

func TestExchangeCode_CodigoValido_TokensDistintosEFakeSoRecebeHashes(t *testing.T) {
	repo := testRepositoryWithRedeemableCode()
	grant := validCodeGrant()

	response, err := newTestService(repo).ExchangeCode(context.Background(), grant)

	require.NoError(t, err)
	secrets := []string{grant.Code, response.AccessToken, response.RefreshToken}
	distinct := map[string]bool{secrets[0]: true, secrets[1]: true, secrets[2]: true}
	assert.Len(t, distinct, 3, "código, access e refresh devem ser distintos")
	require.Len(t, repo.saved, 1)
	require.Len(t, repo.saved[0], 2)
	for _, token := range repo.saved[0] {
		for _, secret := range secrets {
			assert.NotContains(t, fmt.Sprintf("%+v", token), secret, "token em claro chegou ao repositório")
		}
	}
	assert.Equal(t, auth.HashOpaqueToken(response.AccessToken), repo.saved[0][0].Hash)
	assert.Equal(t, auth.HashOpaqueToken(response.RefreshToken), repo.saved[0][1].Hash)
}
