package oauth_test

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

const (
	testIssuer         = "https://markupp.exemplo.dev"
	testPanelURL       = "https://painel.exemplo.dev"
	testResource       = "https://markupp.exemplo.dev/mcp"
	testClientID       = "https://cliente.exemplo.dev/oauth.json"
	testRedirectURI    = "https://cliente.exemplo.dev/callback"
	testVerifier       = "verificador-de-teste-com-mais-de-quarenta-e-tres-caracteres"
	testResourceSecret = "segredo-do-resource-com-32-bytes" // #nosec G101 -- valor fictício de teste
)

var testNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.FixedZone("BRT", -3*3600))

func testServiceConfig() oauth.ServiceConfig {
	return oauth.ServiceConfig{
		Issuer:          testIssuer,
		PanelURL:        testPanelURL,
		Resources:       []string{testResource},
		AccessTTL:       time.Hour,
		RefreshTTL:      30 * 24 * time.Hour,
		RequestTTL:      10 * time.Minute,
		CodeTTL:         time.Minute,
		ResourceSecrets: map[string]string{testResource: testResourceSecret},
		ExchangeTTL:     5 * time.Minute,
		DCRPendingTTL:   testDCRPendingTTL,
	}
}

// sequenceSource devolve valores previsíveis com um prefixo e um contador.
type sequenceSource struct {
	prefix string
	count  int
}

func (s *sequenceSource) next() string {
	s.count++
	return fmt.Sprintf("%s-%d", s.prefix, s.count)
}

// newTestService monta o Service com relógio fixo, IDs "pedido-N" e tokens "token-N".
func newTestService(repo *fakeOAuthRepository) *oauth.Service {
	return newTestServiceWithIssuer(repo, nil)
}

// newTestServiceWithIssuer é o newTestService com o emissor do token da troca.
func newTestServiceWithIssuer(repo *fakeOAuthRepository, scoped oauth.ScopedTokenIssuer) *oauth.Service {
	ids := &sequenceSource{prefix: "pedido"}
	tokens := &sequenceSource{prefix: "token"}
	clock := func() time.Time { return testNow }
	return oauth.NewService(repo, testServiceConfig(), scoped, nil, clock, ids.next, tokens.next)
}

func testChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func testClient() oauth.Client {
	return oauth.Client{ID: testClientID, Name: "Cliente de teste", RedirectURIs: []string{testRedirectURI}}
}

func testRepositoryWithClient() *fakeOAuthRepository {
	repo := newFakeOAuthRepository()
	repo.clients[testClientID] = testClient()
	return repo
}

// requireProtocolError confere o código e o destino do erro de protocolo, e que a
// descrição diz o formato esperado.
func requireProtocolError(t *testing.T, err error, code, redirectURI, state string) {
	t.Helper()
	var protocolErr *oauth.ProtocolError
	require.True(t, errors.As(err, &protocolErr), "esperava *oauth.ProtocolError, veio %v", err)
	assert.Equal(t, code, protocolErr.Code)
	assert.Equal(t, redirectURI, protocolErr.RedirectURI)
	assert.Equal(t, state, protocolErr.State)
	assert.Contains(t, protocolErr.Description, "; esperad", "a descrição deve dizer o formato esperado")
}

// requireNotProtocolError confere que a falha de infraestrutura não virou erro de protocolo.
func requireNotProtocolError(t *testing.T, err error, cause error) {
	t.Helper()
	var protocolErr *oauth.ProtocolError
	require.Error(t, err)
	assert.False(t, errors.As(err, &protocolErr), "falha de repositório virou erro de protocolo: %v", err)
	assert.ErrorIs(t, err, cause)
}
