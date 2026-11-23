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

const (
	segredoDoPainel   = "segredo-de-relay-do-painel-com-32-bytes" // #nosec G101 -- valor fictício de teste
	visitanteDoPainel = "203.0.113.1"
	outroVisitante    = "203.0.113.2"
)

// routerPublicoComRelays monta a leitura pública com o limite de um pedido por minuto e os
// relays dados.
func routerPublicoComRelays(relay api.RelayAuthenticator) http.Handler {
	clock := &fakeRateClock{current: time.Unix(1_000_000, 0)}
	return api.NewRouter(api.RouterDeps{
		Tokens:            &fakeTokenValidator{err: auth.ErrInvalidToken},
		PublicNotes:       &fakePublicNoteReader{},
		PublicRateLimiter: api.NewAccountRateLimiter(limiteDeUmPorMinuto, time.Minute, clock.Now, nil),
		PublicRelay:       relay,
	})
}

// pedidoComBasic é o pedido de um relay com o X-Forwarded-For e o HTTP Basic dados.
func pedidoComBasic(forwardedFor, usuario, senha string) map[string]string {
	basic := url.QueryEscape(usuario) + ":" + url.QueryEscape(senha)
	return map[string]string{
		"X-Forwarded-For": forwardedFor,
		"Authorization":   "Basic " + base64.StdEncoding.EncodeToString([]byte(basic)),
	}
}

// contaPorVisitante diz se o segundo visitante pelo mesmo relay ainda tem limite próprio.
func contaPorVisitante(t *testing.T, router http.Handler, usuario, senha string) bool {
	t.Helper()
	require.Equal(t, http.StatusOK, getPublico(router, notaPublicaPath, pedidoComBasic(visitanteDoPainel, usuario, senha)).Code)
	return getPublico(router, notaPublicaPath, pedidoComBasic(outroVisitante, usuario, senha)).Code == http.StatusOK
}

func TestPanelRelay_SemMCPNemOAuth_ContaOLimitePeloVisitante(t *testing.T) {
	router := routerPublicoComRelays(api.RelayAuthenticators{api.PanelRelay{Secret: segredoDoPainel}})

	assert.True(t, contaPorVisitante(t, router, api.PanelRelayID, segredoDoPainel))
}

func TestPanelRelay_SegredoErrado_ContaPeloIPDaConexao(t *testing.T) {
	router := routerPublicoComRelays(api.PanelRelay{Secret: segredoDoPainel})

	assert.False(t, contaPorVisitante(t, router, api.PanelRelayID, "segredo-errado"))
}

func TestPanelRelay_SemSegredoConfigurado_NaoAceitaSenhaVazia(t *testing.T) {
	router := routerPublicoComRelays(api.PanelRelay{})

	assert.False(t, contaPorVisitante(t, router, api.PanelRelayID, ""))
}

func TestRelayAuthenticators_SegredoDoMCPNaoAutenticaComoPainel(t *testing.T) {
	relays := api.RelayAuthenticators{
		api.PanelRelay{Secret: segredoDoPainel},
		fakeRelayAuthenticator{resourceID: relayDeTeste, secret: segredoDoRelay},
	}

	assert.False(t, relays.AuthenticatesResource(api.PanelRelayID, segredoDoRelay))
	assert.False(t, relays.AuthenticatesResource(relayDeTeste, segredoDoPainel))
	assert.True(t, relays.AuthenticatesResource(api.PanelRelayID, segredoDoPainel))
	assert.True(t, relays.AuthenticatesResource(relayDeTeste, segredoDoRelay))
}
