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

const (
	testDCRMaxPending = 7
	testDCRPendingTTL = 24 * time.Hour
)

// newDCRTestService é o newTestService com o teto e o prazo dos pendentes do DCR.
func newDCRTestService(repo *fakeOAuthRepository) *oauth.Service {
	cfg := testServiceConfig()
	cfg.DCRMaxPending, cfg.DCRPendingTTL = testDCRMaxPending, testDCRPendingTTL
	tokens := &sequenceSource{prefix: "token"}
	clock := func() time.Time { return testNow }
	return oauth.NewService(repo, cfg, nil, nil, clock, (&sequenceSource{prefix: "pedido"}).next, tokens.next)
}

func validRegistration() oauth.RegistrationRequest {
	return oauth.RegistrationRequest{
		ClientName:   "Agente DCR",
		RedirectURIs: []string{"http://127.0.0.1:51234/cb", "meuapp://callback"},
	}
}

func register(t *testing.T, repo *fakeOAuthRepository, req oauth.RegistrationRequest) (oauth.RegistrationResponse, error) {
	t.Helper()
	return newDCRTestService(repo).RegisterClient(context.Background(), req)
}

func TestRegisterClient_Valido_GravaClientePublicoEDevolveOsValoresEfetivos(t *testing.T) {
	repo := newFakeOAuthRepository()

	got, err := register(t, repo, validRegistration())

	require.NoError(t, err)
	assert.Equal(t, oauth.RegistrationResponse{
		ClientID: "markupp-dcr-token-1", ClientIDIssuedAt: testNow.Unix(), ClientName: "Agente DCR",
		RedirectURIs:            []string{"http://127.0.0.1:51234/cb", "meuapp://callback"},
		TokenEndpointAuthMethod: "none",
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
	}, got)
	require.Len(t, repo.dcrRegistrations, 1)
	assert.Equal(t, oauth.DCRRegistration{
		Client:   oauth.Client{ID: "markupp-dcr-token-1", Name: "Agente DCR", RedirectURIs: got.RedirectURIs, Source: oauth.ClientSourceDCR},
		IssuedAt: testNow, MaxPending: testDCRMaxPending, PendingTTL: testDCRPendingTTL,
	}, repo.dcrRegistrations[0])
}

func TestRegisterClient_ClientIDNuncaEURLHTTPS(t *testing.T) {
	got, err := register(t, newFakeOAuthRepository(), validRegistration())

	require.NoError(t, err)
	assert.False(t, strings.HasPrefix(got.ClientID, "https:"), got.ClientID)
	assert.NotContains(t, got.ClientID, "://")
}

func TestRegisterClient_MetadadosDeClientePublicoAceitos(t *testing.T) {
	casos := map[string]func(*oauth.RegistrationRequest){
		"auth method none":        func(r *oauth.RegistrationRequest) { r.TokenEndpointAuthMethod = new("none") },
		"grant authorization":     func(r *oauth.RegistrationRequest) { r.GrantTypes = []string{"authorization_code"} },
		"grants suportados":       func(r *oauth.RegistrationRequest) { r.GrantTypes = []string{"refresh_token", "authorization_code"} },
		"grant_types vazio":       func(r *oauth.RegistrationRequest) { r.GrantTypes = []string{} },
		"response_types code":     func(r *oauth.RegistrationRequest) { r.ResponseTypes = []string{"code"} },
		"nome com 100 caracteres": func(r *oauth.RegistrationRequest) { r.ClientName = strings.Repeat("á", 100) },
	}
	for nome, alterar := range casos {
		t.Run(nome, func(t *testing.T) {
			req := validRegistration()
			alterar(&req)

			got, err := register(t, newFakeOAuthRepository(), req)

			require.NoError(t, err)
			assert.Equal(t, []string{"authorization_code", "refresh_token"}, got.GrantTypes)
			assert.Equal(t, "none", got.TokenEndpointAuthMethod)
		})
	}
}

func TestRegisterClient_MetadadoInvalido_RecusaSemGravar(t *testing.T) {
	casos := map[string]func(*oauth.RegistrationRequest){
		"auth method com segredo": func(r *oauth.RegistrationRequest) { r.TokenEndpointAuthMethod = new("client_secret_basic") },
		"auth method vazio":       func(r *oauth.RegistrationRequest) { r.TokenEndpointAuthMethod = new("") },
		"grant implicito":         func(r *oauth.RegistrationRequest) { r.GrantTypes = []string{"implicit"} },
		"grant client_credentials": func(r *oauth.RegistrationRequest) {
			r.GrantTypes = []string{"authorization_code", "client_credentials"}
		},
		"response_types token":      func(r *oauth.RegistrationRequest) { r.ResponseTypes = []string{"token"} },
		"response_types code token": func(r *oauth.RegistrationRequest) { r.ResponseTypes = []string{"code", "token"} },
		"response_types vazio":      func(r *oauth.RegistrationRequest) { r.ResponseTypes = []string{} },
		"nome ausente":              func(r *oauth.RegistrationRequest) { r.ClientName = "" },
		"nome em branco":            func(r *oauth.RegistrationRequest) { r.ClientName = " \t\n\u200b " },
		"nome com 101 caracteres":   func(r *oauth.RegistrationRequest) { r.ClientName = strings.Repeat("a", 101) },
		"nome com UTF-8 invalido":   func(r *oauth.RegistrationRequest) { r.ClientName = "Agente \xff" },
		"nome zalgo":                func(r *oauth.RegistrationRequest) { r.ClientName = "Z\u0351\u0352\u0353algo" },
	}
	for nome, alterar := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := newFakeOAuthRepository()
			req := validRegistration()
			alterar(&req)

			_, err := register(t, repo, req)

			requireProtocolError(t, err, "invalid_client_metadata", "", "")
			assert.Empty(t, repo.dcrRegistrations)
		})
	}
}

func TestRegisterClient_NomeComControleEFormatacao_GravaSaneado(t *testing.T) {
	repo := newFakeOAuthRepository()
	req := validRegistration()
	req.ClientName = "  Agente\u202e \n\tBom\x00 "

	got, err := register(t, repo, req)

	require.NoError(t, err)
	assert.Equal(t, "Agente Bom", got.ClientName)
	assert.Equal(t, "Agente Bom", repo.dcrRegistrations[0].Client.Name)
}

func TestRegisterClient_RedirectURIInvalida_RecusaComInvalidRedirectURI(t *testing.T) {
	casos := map[string][]string{
		"ausente":         nil,
		"vazia":           {},
		"javascript":      {"https://agente.example/cb", "javascript:alert(1)"},
		"http na rede":    {"http://agente.example/cb"},
		"com fragmento":   {"https://agente.example/cb#x"},
		"com credenciais": {"https://usuario@agente.example/cb"},
		"relativa":        {"/cb"},
	}
	for nome, uris := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := newFakeOAuthRepository()
			req := validRegistration()
			req.RedirectURIs = uris

			_, err := register(t, repo, req)

			requireProtocolError(t, err, "invalid_redirect_uri", "", "")
			assert.Empty(t, repo.dcrRegistrations)
		})
	}
}

func TestRegisterClient_RedirectURILonga_ChegaLimitadaADescricao(t *testing.T) {
	longa := "javascript:" + strings.Repeat("a", 500)
	req := validRegistration()
	req.RedirectURIs = []string{longa}

	_, err := register(t, newFakeOAuthRepository(), req)

	var protocolErr *oauth.ProtocolError
	require.ErrorAs(t, err, &protocolErr)
	assert.NotContains(t, protocolErr.Description, longa)
	assert.Contains(t, protocolErr.Description, "javascript:aaa")
}

func TestRegisterClient_TetoDePendentes_RecusaTemporariamenteComRetryAfter(t *testing.T) {
	casos := map[string]struct {
		freesAt time.Time
		retry   time.Duration
	}{
		"vaga no futuro":  {testNow.Add(90 * time.Minute), 90 * time.Minute},
		"vaga ja passada": {testNow.Add(-time.Minute), time.Second},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := newFakeOAuthRepository()
			repo.registerDCRErr = &oauth.PendingClientLimitError{MaxPending: testDCRMaxPending, FreesAt: caso.freesAt}

			_, err := register(t, repo, validRegistration())

			requireProtocolError(t, err, "temporarily_unavailable", "", "")
			var protocolErr *oauth.ProtocolError
			require.ErrorAs(t, err, &protocolErr)
			assert.Equal(t, caso.retry, protocolErr.RetryAfter)
		})
	}
}

func TestRegisterClient_FalhaDoRepositorio_NaoViraErroDeProtocolo(t *testing.T) {
	repo := newFakeOAuthRepository()
	repo.registerDCRErr = errors.New("banco fora do ar")

	_, err := register(t, repo, validRegistration())

	requireNotProtocolError(t, err, repo.registerDCRErr)
}
