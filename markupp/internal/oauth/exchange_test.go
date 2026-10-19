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

const testScopedToken = "jwt-curto-da-api" // #nosec G101 -- valor fictício de teste

// scopedIssue é uma chamada a IssueScopedToken.
type scopedIssue struct {
	UserID string
	Scopes []string
	TTL    time.Duration
}

// fakeScopedTokenIssuer guarda as emissões pedidas e devolve testScopedToken, ou err.
type fakeScopedTokenIssuer struct {
	issues []scopedIssue
	err    error
}

func (f *fakeScopedTokenIssuer) IssueScopedToken(userID string, scopes []string, ttl time.Duration) (string, error) {
	f.issues = append(f.issues, scopedIssue{UserID: userID, Scopes: scopes, TTL: ttl})
	if f.err != nil {
		return "", f.err
	}
	return testScopedToken, nil
}

// testAccessGrant é o access token vigente da aut-1 para testResource.
func testAccessGrant() oauth.TokenGrant {
	grant := testTokenGrant()
	grant.Token.Hash = auth.HashOpaqueToken(testAccessToken)
	grant.Token.Kind = oauth.TokenKindAccess
	return grant
}

func testRepositoryWithAccessToken() *fakeOAuthRepository {
	repo := newFakeOAuthRepository()
	repo.found = testAccessGrant()
	return repo
}

func validExchangeGrant() oauth.ExchangeGrant {
	return oauth.ExchangeGrant{
		ResourceID: testResource, ResourceSecret: testResourceSecret,
		SubjectToken: testAccessToken, SubjectTokenType: oauth.TokenTypeAccessToken,
	}
}

// exchange roda a troca com um repositório de access token vigente, alterado por change.
func exchange(t *testing.T, g oauth.ExchangeGrant, change func(*fakeOAuthRepository)) (*fakeOAuthRepository, *fakeScopedTokenIssuer, oauth.ExchangeResponse, error) {
	t.Helper()
	repo, issuer := testRepositoryWithAccessToken(), &fakeScopedTokenIssuer{}
	if change != nil {
		change(repo)
	}
	response, err := newTestServiceWithIssuer(repo, issuer).ExchangeToken(context.Background(), g)
	return repo, issuer, response, err
}

func TestExchangeToken_TokenDoResource_EmiteOTokenCurtoEAudita(t *testing.T) {
	repo, issuer, response, err := exchange(t, validExchangeGrant(), nil)

	require.NoError(t, err)
	assert.Equal(t, oauth.ExchangeResponse{
		AccessToken: testScopedToken, IssuedTokenType: oauth.TokenTypeAccessToken,
		TokenType: "Bearer", ExpiresIn: 300, Scope: "notas:leitura notas:escrita",
	}, response)
	assert.Equal(t, []string{auth.HashOpaqueToken(testAccessToken)}, repo.findCalls)
	assert.Equal(t, []scopedIssue{{
		UserID: testUserID, Scopes: []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite}, TTL: 5 * time.Minute,
	}}, issuer.issues)
	assert.Equal(t, []oauth.AuditEvent{testAuditEvent(oauth.AuditActionExchange, oauth.AuditResultSuccess)}, repo.recordedEvents)
}

func TestExchangeToken_ParametrosOpcionaisEquivalentes_Aceita(t *testing.T) {
	casos := map[string]func(*oauth.ExchangeGrant){
		"resource id com maiusculas e barra": func(g *oauth.ExchangeGrant) { g.ResourceID = "https://MARKUPP.exemplo.dev/mcp/" },
		"requested_token_type access_token":  func(g *oauth.ExchangeGrant) { g.RequestedTokenType = oauth.TokenTypeAccessToken },
		"audience igual ao issuer":           func(g *oauth.ExchangeGrant) { g.Audience = "https://Markupp.exemplo.dev/" },
		"resource igual ao issuer":           func(g *oauth.ExchangeGrant) { g.Resource = testIssuer },
	}
	for nome, altera := range casos {
		t.Run(nome, func(t *testing.T) {
			grant := validExchangeGrant()
			altera(&grant)

			_, _, response, err := exchange(t, grant, nil)

			require.NoError(t, err)
			assert.Equal(t, testScopedToken, response.AccessToken)
		})
	}
}

func TestExchangeToken_ResourceNaoAutenticado_InvalidClientSemBuscar(t *testing.T) {
	casos := map[string]func(*oauth.ExchangeGrant){
		"resource id vazio":        func(g *oauth.ExchangeGrant) { g.ResourceID = "" },
		"resource desconhecido":    func(g *oauth.ExchangeGrant) { g.ResourceID = "https://outro.exemplo.dev/mcp" },
		"resource id ilegivel":     func(g *oauth.ExchangeGrant) { g.ResourceID = "http://[::1" },
		"segredo vazio":            func(g *oauth.ExchangeGrant) { g.ResourceSecret = "" },
		"segredo errado":           func(g *oauth.ExchangeGrant) { g.ResourceSecret = "segredo-do-resource-com-32-bytez" },
		"prefixo do segredo":       func(g *oauth.ExchangeGrant) { g.ResourceSecret = testResourceSecret[:10] },
		"segredo de outro tamanho": func(g *oauth.ExchangeGrant) { g.ResourceSecret = testResourceSecret + "x" },
	}
	for nome, altera := range casos {
		t.Run(nome, func(t *testing.T) {
			grant := validExchangeGrant()
			altera(&grant)

			repo, issuer, _, err := exchange(t, grant, nil)

			requireProtocolError(t, err, "invalid_client", "", "")
			assertSecretsAbsent(t, err, testResourceSecret)
			if grant.ResourceSecret != "" {
				assertSecretsAbsent(t, err, grant.ResourceSecret)
			}
			assert.Empty(t, repo.findCalls)
			assert.Empty(t, issuer.issues)
		})
	}
}

func TestExchangeToken_ParametroInvalido_InvalidRequestSemBuscar(t *testing.T) {
	casos := map[string]func(*oauth.ExchangeGrant){
		"subject_token vazio":            func(g *oauth.ExchangeGrant) { g.SubjectToken = "" },
		"subject_token_type vazio":       func(g *oauth.ExchangeGrant) { g.SubjectTokenType = "" },
		"subject_token_type de refresh":  func(g *oauth.ExchangeGrant) { g.SubjectTokenType = "urn:ietf:params:oauth:token-type:refresh_token" },
		"requested_token_type diferente": func(g *oauth.ExchangeGrant) { g.RequestedTokenType = "urn:ietf:params:oauth:token-type:jwt" },
	}
	for nome, altera := range casos {
		t.Run(nome, func(t *testing.T) {
			grant := validExchangeGrant()
			altera(&grant)

			repo, _, _, err := exchange(t, grant, nil)

			requireProtocolError(t, err, "invalid_request", "", "")
			assert.Empty(t, repo.findCalls)
		})
	}
}

func TestExchangeToken_AlvoQueNaoEOIssuer_InvalidTargetSemBuscar(t *testing.T) {
	casos := map[string]func(*oauth.ExchangeGrant){
		"audience do resource": func(g *oauth.ExchangeGrant) { g.Audience = testResource },
		"resource de outro":    func(g *oauth.ExchangeGrant) { g.Resource = "https://outro.exemplo.dev" },
		"audience ilegivel":    func(g *oauth.ExchangeGrant) { g.Audience = "http://[::1" },
	}
	for nome, altera := range casos {
		t.Run(nome, func(t *testing.T) {
			grant := validExchangeGrant()
			altera(&grant)

			repo, _, _, err := exchange(t, grant, nil)

			requireProtocolError(t, err, "invalid_target", "", "")
			assert.Empty(t, repo.findCalls)
		})
	}
}

func TestExchangeToken_SubjectTokenQueNaoVale_InvalidGrantSemEmitir(t *testing.T) {
	casos := map[string]func(*fakeOAuthRepository){
		"inexistente":          func(r *fakeOAuthRepository) { r.found = oauth.TokenGrant{} },
		"refresh token":        func(r *fakeOAuthRepository) { r.found.Token.Kind = oauth.TokenKindRefresh },
		"revogado":             func(r *fakeOAuthRepository) { r.found.Token.Revoked = true },
		"expirado agora":       func(r *fakeOAuthRepository) { r.found.Token.ExpiresAt = testNow },
		"autorizacao revogada": func(r *fakeOAuthRepository) { r.found.Authorization.Revoked = true },
		"de outro resource":    func(r *fakeOAuthRepository) { r.found.Token.Resource = "https://outro.exemplo.dev/mcp" },
	}
	for nome, altera := range casos {
		t.Run(nome, func(t *testing.T) {
			repo, issuer, _, err := exchange(t, validExchangeGrant(), altera)

			requireProtocolError(t, err, "invalid_grant", "", "")
			assertSecretsAbsent(t, err, testAccessToken, auth.HashOpaqueToken(testAccessToken))
			assert.Empty(t, issuer.issues)
			assert.Empty(t, repo.recordedEvents)
		})
	}
}

func TestExchangeToken_FalhaDeInfraestrutura_DevolveErroEmbrulhado(t *testing.T) {
	causa := errors.New("falha simulada")
	casos := map[string]func(*fakeOAuthRepository, *fakeScopedTokenIssuer){
		"busca do token": func(r *fakeOAuthRepository, _ *fakeScopedTokenIssuer) { r.findErr = causa },
		"emissao do jwt": func(_ *fakeOAuthRepository, i *fakeScopedTokenIssuer) { i.err = causa },
		"auditoria":      func(r *fakeOAuthRepository, _ *fakeScopedTokenIssuer) { r.recordErr = causa },
	}
	for nome, altera := range casos {
		t.Run(nome, func(t *testing.T) {
			repo, issuer := testRepositoryWithAccessToken(), &fakeScopedTokenIssuer{}
			altera(repo, issuer)

			response, err := newTestServiceWithIssuer(repo, issuer).ExchangeToken(context.Background(), validExchangeGrant())

			requireNotProtocolError(t, err, causa)
			assert.Empty(t, response.AccessToken)
			assertSecretsAbsent(t, err, testAccessToken, testResourceSecret, testScopedToken)
		})
	}
}

func TestExchangeToken_FalhaNaEmissao_NaoAudita(t *testing.T) {
	repo, issuer := testRepositoryWithAccessToken(), &fakeScopedTokenIssuer{err: errors.New("falha simulada")}

	_, err := newTestServiceWithIssuer(repo, issuer).ExchangeToken(context.Background(), validExchangeGrant())

	require.Error(t, err)
	assert.Empty(t, repo.recordedEvents)
}
