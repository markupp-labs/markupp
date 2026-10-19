package oauth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func validAuthorizeParams() oauth.AuthorizeParams {
	return oauth.AuthorizeParams{
		ResponseType:        "code",
		ClientID:            testClientID,
		RedirectURI:         testRedirectURI,
		Scope:               "notas:escrita notas:leitura",
		State:               "estado-do-cliente",
		CodeChallenge:       testChallenge(testVerifier),
		CodeChallengeMethod: "S256",
		Resource:            "HTTPS://Markupp.Exemplo.dev/mcp/",
	}
}

func TestStartAuthorization_PedidoValido_CriaPedidoEDevolveConsentimento(t *testing.T) {
	repo := testRepositoryWithClient()

	consentURL, err := newTestService(repo).StartAuthorization(context.Background(), validAuthorizeParams())

	require.NoError(t, err)
	assert.Equal(t, testPanelURL+"/oauth/consentimento?pedido=pedido-1", consentURL)
	require.Len(t, repo.created, 1)
	assert.Equal(t, oauth.AuthorizationRequest{
		ID:            "pedido-1",
		ClientID:      testClientID,
		RedirectURI:   testRedirectURI,
		Scopes:        []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite},
		Resource:      testResource,
		State:         "estado-do-cliente",
		CodeChallenge: testChallenge(testVerifier),
		CreatedAt:     testNow,
		ExpiresAt:     testNow.Add(testServiceConfig().RequestTTL),
	}, repo.created[0])
}

func TestStartAuthorization_IDComCaracteresEspeciais_EscapaNaQuery(t *testing.T) {
	repo := testRepositoryWithClient()
	clock := func() time.Time { return testNow }
	service := oauth.NewService(repo, testServiceConfig(), nil, nil, clock, func() string { return "a b&c" }, func() string { return "t" })

	consentURL, err := service.StartAuthorization(context.Background(), validAuthorizeParams())

	require.NoError(t, err)
	assert.Equal(t, testPanelURL+"/oauth/consentimento?pedido=a+b%26c", consentURL)
}

func TestStartAuthorization_Escopos_NormalizaPelaOrdemSuportada(t *testing.T) {
	casos := map[string]struct {
		scope    string
		esperado []string
	}{
		"vazio vira leitura":    {"", []string{oauth.ScopeNotesRead}},
		"so espacos":            {"   ", []string{oauth.ScopeNotesRead}},
		"repetido":              {"notas:leitura notas:leitura", []string{oauth.ScopeNotesRead}},
		"fora de ordem":         {"administracao notas:leitura", []string{oauth.ScopeNotesRead, oauth.ScopeAdmin}},
		"todos com repeticao":   {"administracao notas:escrita administracao notas:leitura", oauth.SupportedScopes()},
		"espacos entre escopos": {" notas:escrita  notas:leitura ", []string{oauth.ScopeNotesRead, oauth.ScopeNotesWrite}},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithClient()
			params := validAuthorizeParams()
			params.Scope = caso.scope

			_, err := newTestService(repo).StartAuthorization(context.Background(), params)

			require.NoError(t, err)
			require.Len(t, repo.created, 1)
			assert.Equal(t, caso.esperado, repo.created[0].Scopes)
		})
	}
}

func TestStartAuthorization_ClienteOuRedirectInvalidos_ErroSemRedirect(t *testing.T) {
	casos := map[string]func(*oauth.AuthorizeParams){
		"client_id vazio":          func(p *oauth.AuthorizeParams) { p.ClientID = "" },
		"cliente desconhecido":     func(p *oauth.AuthorizeParams) { p.ClientID = "https://outro.dev/oauth.json" },
		"redirect_uri vazia":       func(p *oauth.AuthorizeParams) { p.RedirectURI = "" },
		"redirect_uri diferente":   func(p *oauth.AuthorizeParams) { p.RedirectURI = "https://cliente.exemplo.dev/outra" },
		"redirect_uri a mais":      func(p *oauth.AuthorizeParams) { p.RedirectURI = testRedirectURI + "/" },
		"redirect com fragmento":   func(p *oauth.AuthorizeParams) { p.RedirectURI = testRedirectURI + "#x" },
		"redirect invalida e tipo": func(p *oauth.AuthorizeParams) { p.RedirectURI = "x"; p.ResponseType = "token" },
	}
	for nome, altera := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithClient()
			params := validAuthorizeParams()
			altera(&params)

			_, err := newTestService(repo).StartAuthorization(context.Background(), params)

			requireProtocolError(t, err, "invalid_request", "", "")
			assert.Empty(t, repo.created)
		})
	}
}

func TestStartAuthorization_ParametroInvalido_ErroComRedirect(t *testing.T) {
	casos := map[string]struct {
		altera func(*oauth.AuthorizeParams)
		code   string
	}{
		"response_type token":   {func(p *oauth.AuthorizeParams) { p.ResponseType = "token" }, "unsupported_response_type"},
		"response_type vazio":   {func(p *oauth.AuthorizeParams) { p.ResponseType = "" }, "unsupported_response_type"},
		"metodo plain":          {func(p *oauth.AuthorizeParams) { p.CodeChallengeMethod = "plain" }, "invalid_request"},
		"metodo vazio":          {func(p *oauth.AuthorizeParams) { p.CodeChallengeMethod = "" }, "invalid_request"},
		"metodo em minusculas":  {func(p *oauth.AuthorizeParams) { p.CodeChallengeMethod = "s256" }, "invalid_request"},
		"challenge vazio":       {func(p *oauth.AuthorizeParams) { p.CodeChallenge = "" }, "invalid_request"},
		"challenge curto":       {func(p *oauth.AuthorizeParams) { p.CodeChallenge = "abc" }, "invalid_request"},
		"resource vazio":        {func(p *oauth.AuthorizeParams) { p.Resource = "" }, "invalid_target"},
		"resource ilegivel":     {func(p *oauth.AuthorizeParams) { p.Resource = "http://[::1" }, "invalid_target"},
		"resource desconhecido": {func(p *oauth.AuthorizeParams) { p.Resource = "https://outro.dev/mcp" }, "invalid_target"},
		"resource relativo":     {func(p *oauth.AuthorizeParams) { p.Resource = "/mcp" }, "invalid_target"},
		"escopo desconhecido":   {func(p *oauth.AuthorizeParams) { p.Scope = "notas:leitura tudo" }, "invalid_scope"},
		"escopo em maiusculas":  {func(p *oauth.AuthorizeParams) { p.Scope = "NOTAS:LEITURA" }, "invalid_scope"},
		"escopo com tabulacao":  {func(p *oauth.AuthorizeParams) { p.Scope = "notas:leitura\tadministracao" }, "invalid_scope"},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := testRepositoryWithClient()
			params := validAuthorizeParams()
			caso.altera(&params)

			_, err := newTestService(repo).StartAuthorization(context.Background(), params)

			requireProtocolError(t, err, caso.code, testRedirectURI, "estado-do-cliente")
			assert.Empty(t, repo.created)
		})
	}
}

func TestStartAuthorization_FalhaAoCriar_DevolveErroEmbrulhado(t *testing.T) {
	repo := testRepositoryWithClient()
	repo.createErr = errors.New("banco fora do ar")

	_, err := newTestService(repo).StartAuthorization(context.Background(), validAuthorizeParams())

	requireNotProtocolError(t, err, repo.createErr)
}

func TestStartAuthorization_ValorRecebidoNaDescricao_FicaNoConjuntoDaRFC6749(t *testing.T) {
	repo := testRepositoryWithClient()
	params := validAuthorizeParams()
	params.Scope = "notas:leitura \"ação\\\x01" + strings.Repeat("x", 200)

	_, err := newTestService(repo).StartAuthorization(context.Background(), params)

	var protocolErr *oauth.ProtocolError
	require.True(t, errors.As(err, &protocolErr))
	assert.Contains(t, protocolErr.Description, "'?a??o??xxx")
	assert.Less(t, len(protocolErr.Description), 250)
	for _, c := range protocolErr.Description {
		assert.True(t, c >= 0x20 && c <= 0x7e && c != '"' && c != '\\', "caractere %q fora do error_description permitido", c)
	}
}

func TestStartAuthorization_SemResourcesConfigurados_DescricaoCompleta(t *testing.T) {
	cfg := testServiceConfig()
	cfg.Resources = nil
	service := oauth.NewService(testRepositoryWithClient(), cfg, nil, nil, func() time.Time { return testNow }, nil, nil)

	_, err := service.StartAuthorization(context.Background(), validAuthorizeParams())

	var protocolErr *oauth.ProtocolError
	require.True(t, errors.As(err, &protocolErr))
	assert.Equal(t, "invalid_target", protocolErr.Code)
	assert.NotContains(t, protocolErr.Description, "um de ")
	assert.Contains(t, protocolErr.Description, "nenhum servidor protegido configurado")
}
