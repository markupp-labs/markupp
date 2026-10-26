package api_test

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// fakeRelayAuthenticator aceita só o par de resource e segredo dado, como o Go confere o
// servidor MCP.
type fakeRelayAuthenticator struct {
	resourceID, secret string
}

func (f fakeRelayAuthenticator) AuthenticatesResource(resourceID, secret string) bool {
	return resourceID == f.resourceID && secret == f.secret
}

const (
	relayDeTeste        = "http://localhost:8080/mcp"
	segredoDoRelay      = "segredo-do-mcp" // #nosec G101 -- valor fictício de teste
	cofreDocsDoRelay    = "/v1/public/vaults/docs"
	agenteDoRelay       = "198.51.100.1"
	outroAgenteDoRelay  = "198.51.100.2"
	segredoErradoRelay  = "segredo-errado" // #nosec G101 -- valor fictício de teste
	limiteDeUmPorMinuto = 1
)

// routerPublicoComRelay monta a leitura pública com o servidor MCP como relay confiável e
// o limite de um pedido por minuto.
func routerPublicoComRelay() http.Handler {
	clock := &fakeRateClock{current: time.Unix(1_000_000, 0)}
	return api.NewRouter(api.RouterDeps{
		Tokens:            &fakeTokenValidator{err: auth.ErrInvalidToken},
		PublicVaults:      cofrePublicoDeTeste(),
		PublicRateLimiter: api.NewAccountRateLimiter(limiteDeUmPorMinuto, time.Minute, clock.Now, nil),
		PublicRelay:       fakeRelayAuthenticator{resourceID: relayDeTeste, secret: segredoDoRelay},
	})
}

// pedidoDoRelay é o pedido do MCP ao Go em nome do agente: o X-Forwarded-For que o Go
// escreveu em /mcp/public e o HTTP Basic do resource com a senha dada.
func pedidoDoRelay(forwardedFor, senha string) map[string]string {
	basic := url.QueryEscape(relayDeTeste) + ":" + url.QueryEscape(senha)
	return map[string]string{
		"X-Forwarded-For": forwardedFor,
		"Authorization":   "Basic " + base64.StdEncoding.EncodeToString([]byte(basic)),
	}
}

func TestPublico_RelayAutenticado_ContaOLimitePeloIPDoAgente(t *testing.T) {
	router := routerPublicoComRelay()
	require.Equal(t, http.StatusOK, getPublico(router, cofreDocsDoRelay, pedidoDoRelay(agenteDoRelay, segredoDoRelay)).Code)
	require.Equal(t, http.StatusTooManyRequests, getPublico(router, cofreDocsDoRelay, pedidoDoRelay(agenteDoRelay, segredoDoRelay)).Code)

	rec := getPublico(router, cofreDocsDoRelay, pedidoDoRelay(outroAgenteDoRelay, segredoDoRelay))

	assert.Equal(t, http.StatusOK, rec.Code, "outro agente pelo mesmo MCP tem limite proprio")
}

func TestPublico_RelayAutenticado_AgenteNaoEscolheOIPPelaEsquerdaDoHeader(t *testing.T) {
	router := routerPublicoComRelay()
	require.Equal(t, http.StatusOK, getPublico(router, cofreDocsDoRelay, pedidoDoRelay("192.0.2.1, "+agenteDoRelay, segredoDoRelay)).Code)

	rec := getPublico(router, cofreDocsDoRelay, pedidoDoRelay("192.0.2.2, "+agenteDoRelay, segredoDoRelay))

	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "so o salto que o Go escreveu conta, o resto e do agente")
}

func TestPublico_RelaySemOSegredoCerto_IgnoraOHeader(t *testing.T) {
	router := routerPublicoComRelay()
	require.Equal(t, http.StatusOK, getPublico(router, cofreDocsDoRelay, pedidoDoRelay(agenteDoRelay, segredoErradoRelay)).Code)

	rec := getPublico(router, cofreDocsDoRelay, pedidoDoRelay(outroAgenteDoRelay, segredoErradoRelay))

	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "sem o segredo do MCP vale o IP da conexao")
}

func TestPublico_RelayAutenticadoSemHeader_UsaOIPDaConexao(t *testing.T) {
	router := routerPublicoComRelay()
	require.Equal(t, http.StatusOK, getPublico(router, cofreDocsDoRelay, pedidoDoRelay("", segredoDoRelay)).Code)

	rec := getPublico(router, cofreDocsDoRelay, pedidoDoRelay("lixo", segredoDoRelay))

	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "header vazio ou malformado fica com a conexao")
}
