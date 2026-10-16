package oauth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func testRepositoryWithRedeemableCode() *fakeOAuthRepository {
	repo := newFakeOAuthRepository()
	repo.redeemed = oauth.RedeemedCode{
		Request:       pendingTestRequest(),
		Authorization: oauth.Authorization{ID: "aut-1", UserID: testUserID, ClientID: testClientID, Scopes: oauth.SupportedScopes()},
	}
	return repo
}

func validCodeGrant() oauth.CodeGrant {
	return oauth.CodeGrant{
		Code:         "codigo-1",
		CodeVerifier: testVerifier,
		ClientID:     testClientID,
		RedirectURI:  pendingTestRequest().RedirectURI,
	}
}

func TestExchangeCode_CodigoValido_EmiteTokensDosEscoposDoPedido(t *testing.T) {
	repo := testRepositoryWithRedeemableCode()

	response, err := newTestService(repo).ExchangeCode(context.Background(), validCodeGrant())

	require.NoError(t, err)
	assert.Equal(t, oauth.TokenResponse{
		AccessToken:  "token-1",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "token-2",
		Scope:        "notas:leitura notas:escrita",
	}, response)
	assert.Equal(t, []string{auth.HashOpaqueToken("codigo-1")}, repo.redeemCalls)
	assert.Equal(t, []time.Time{testNow}, repo.redeemNow)
	assert.Equal(t, [][]oauth.IssuedToken{expectedIssuedTokens()}, repo.saved)
}

func expectedIssuedTokens() []oauth.IssuedToken {
	scopes := []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite}
	return []oauth.IssuedToken{
		{
			Hash: auth.HashOpaqueToken("token-1"), AuthorizationID: "aut-1", Kind: oauth.TokenKindAccess,
			Resource: testResource, Scopes: scopes, ExpiresAt: testNow.Add(time.Hour),
		},
		{
			Hash: auth.HashOpaqueToken("token-2"), AuthorizationID: "aut-1", Kind: oauth.TokenKindRefresh,
			Resource: testResource, Scopes: scopes, ExpiresAt: testNow.Add(30 * 24 * time.Hour),
		},
	}
}

func TestExchangeCode_ResourceEquivalente_Aceita(t *testing.T) {
	repo := testRepositoryWithRedeemableCode()
	grant := validCodeGrant()
	grant.Resource = "https://MARKUPP.exemplo.dev/mcp/"

	_, err := newTestService(repo).ExchangeCode(context.Background(), grant)

	require.NoError(t, err)
	assert.Len(t, repo.saved, 1)
}

func TestExchangeCode_GrantQueNaoBateComOPedido_InvalidGrant(t *testing.T) {
	casos := map[string]func(*oauth.CodeGrant){
		"client_id diferente":     func(g *oauth.CodeGrant) { g.ClientID = "https://outro.dev/oauth.json" },
		"client_id vazio":         func(g *oauth.CodeGrant) { g.ClientID = "" },
		"redirect_uri diferente":  func(g *oauth.CodeGrant) { g.RedirectURI = testRedirectURI },
		"redirect_uri vazia":      func(g *oauth.CodeGrant) { g.RedirectURI = "" },
		"verifier errado":         func(g *oauth.CodeGrant) { g.CodeVerifier = testVerifier + "x" },
		"verifier vazio":          func(g *oauth.CodeGrant) { g.CodeVerifier = "" },
		"resource diferente":      func(g *oauth.CodeGrant) { g.Resource = "https://outro.dev/mcp" },
		"resource ilegivel":       func(g *oauth.CodeGrant) { g.Resource = "http://[::1" },
		"resource com outro path": func(g *oauth.CodeGrant) { g.Resource = testResource + "/sub" },
	}
	for nome, altera := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithRedeemableCode()
			grant := validCodeGrant()
			altera(&grant)

			_, err := newTestService(repo).ExchangeCode(context.Background(), grant)

			requireProtocolError(t, err, "invalid_grant", "", "")
			assert.Len(t, repo.redeemCalls, 1, "o código é consumido antes das conferências")
			assert.Empty(t, repo.saved)
			assert.Empty(t, repo.revoked)
		})
	}
}

func TestExchangeCode_CodigoRecusadoPeloRepositorio_InvalidGrant(t *testing.T) {
	casos := map[string]error{
		"inexistente":            oauth.ErrCodeNotFound,
		"inexistente embrulhado": errors.Join(errors.New("contexto"), oauth.ErrCodeNotFound),
		"sem autorizacao ativa":  oauth.ErrAuthorizationNotFound,
	}
	for nome, causa := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := newFakeOAuthRepository()
			repo.redeemErr = causa

			_, err := newTestService(repo).ExchangeCode(context.Background(), validCodeGrant())

			requireProtocolError(t, err, "invalid_grant", "", "")
			assert.Empty(t, repo.saved)
			assert.Empty(t, repo.revoked)
		})
	}
}

func TestExchangeCode_CodigoReusado_RevogaTokensDaAutorizacao(t *testing.T) {
	repo := testRepositoryWithRedeemableCode()
	repo.redeemErr = oauth.ErrCodeAlreadyUsed

	_, err := newTestService(repo).ExchangeCode(context.Background(), validCodeGrant())

	requireProtocolError(t, err, "invalid_grant", "", "")
	assert.Equal(t, []string{"aut-1"}, repo.revoked)
	assert.Empty(t, repo.saved)
}

func TestExchangeCode_CodigoReusadoSemAutorizacao_NaoRevoga(t *testing.T) {
	repo := newFakeOAuthRepository()
	repo.redeemErr = oauth.ErrCodeAlreadyUsed

	_, err := newTestService(repo).ExchangeCode(context.Background(), validCodeGrant())

	requireProtocolError(t, err, "invalid_grant", "", "")
	assert.Empty(t, repo.revoked)
}

func TestExchangeCode_FalhaDoRepositorio_DevolveErroEmbrulhado(t *testing.T) {
	casos := map[string]func(*fakeOAuthRepository) error{
		"ao consumir": func(r *fakeOAuthRepository) error { r.redeemErr = errors.New("banco fora do ar"); return r.redeemErr },
		"ao salvar":   func(r *fakeOAuthRepository) error { r.saveErr = errors.New("banco fora do ar"); return r.saveErr },
		"ao revogar": func(r *fakeOAuthRepository) error {
			r.redeemErr = oauth.ErrCodeAlreadyUsed
			r.revokeErr = errors.New("banco fora do ar")
			return r.revokeErr
		},
	}
	for nome, falha := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithRedeemableCode()
			causa := falha(repo)

			_, err := newTestService(repo).ExchangeCode(context.Background(), validCodeGrant())

			requireNotProtocolError(t, err, causa)
		})
	}
}
