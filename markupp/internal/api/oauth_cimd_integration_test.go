package api_test

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth/cimd"
)

// cacheNegativoDeIntegracao é o tempo em que uma busca falha fica lembrada no teste.
const cacheNegativoDeIntegracao = 5 * time.Minute

// publicadorCIMD é o host do cliente: publica um documento válido em /bom.json e
// responde 500 em /quebrado.json, contando as buscas de cada um.
type publicadorCIMD struct {
	*httptest.Server
	buscasBom, buscasQuebrado atomic.Int32
}

func novoPublicadorCIMD(t *testing.T) *publicadorCIMD {
	t.Helper()
	pub := &publicadorCIMD{}
	pub.Server = httptest.NewTLSServer(http.HandlerFunc(pub.servir))
	t.Cleanup(pub.Close)
	return pub
}

func (p *publicadorCIMD) clienteBom() string      { return p.URL + "/bom.json" }
func (p *publicadorCIMD) clienteQuebrado() string { return p.URL + "/quebrado.json" }

// porta é a porta aleatória do publicador, que o fetcher só aceita se liberada.
func (p *publicadorCIMD) porta(t *testing.T) string {
	u, err := url.Parse(p.URL)
	require.NoError(t, err)
	return u.Port()
}

func (p *publicadorCIMD) servir(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/bom.json" {
		p.buscasQuebrado.Add(1)
		http.Error(w, "falha interna do host do cliente", http.StatusInternalServerError)
		return
	}
	p.buscasBom.Add(1)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=600")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"client_id": p.clienteBom(), "client_name": "Agente CIMD", "redirect_uris": []string{"http://127.0.0.1/cb"},
	})
}

func (p *publicadorCIMD) confiaveis() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(p.Certificate())
	return pool
}

// liberarLoopback deixa o fetcher alcançar o publicador no loopback e mantém o resto
// da política padrão.
func liberarLoopback(ip netip.Addr) error {
	if ip.IsLoopback() {
		return nil
	}
	return cimd.PublicOnly(ip)
}

// fetcherCIMDDeTeste entrega ao serviço o que o cimd.Fetcher busca, como o adaptador
// do cmd/markupp, que fica fora do alcance deste pacote.
type fetcherCIMDDeTeste struct{ fetcher *cimd.Fetcher }

func (f fetcherCIMDDeTeste) FetchClientMetadata(ctx context.Context, clientID string) (oauth.ClientMetadata, error) {
	meta, err := f.fetcher.Fetch(ctx, clientID)
	if err != nil {
		return oauth.ClientMetadata{}, err
	}
	return oauth.ClientMetadata{ClientID: meta.ClientID, Name: meta.Name, RedirectURIs: meta.RedirectURIs, TTL: meta.TTL}, nil
}

func setupCIMDIntegrationServer(t *testing.T) (*httptest.Server, *publicadorCIMD) {
	t.Helper()
	pub := novoPublicadorCIMD(t)
	fetcher, err := cimd.NewFetcher(cimd.WithRootCAs(pub.confiaveis()), cimd.WithIPPolicy(liberarLoopback), cimd.WithAllowedPort(pub.porta(t)))
	require.NoError(t, err)
	server, _ := setupOAuthIntegrationServerWith(t, fetcherCIMDDeTeste{fetcher: fetcher}, cacheNegativoDeIntegracao)
	return server, pub
}

func TestIntegration_OAuthCIMD_FluxoCompletoComDocumentoEmCache(t *testing.T) {
	server, pub := setupCIMDIntegrationServer(t)
	accountToken := loginDeIntegracao(t, server)

	retorno := aprovarPedido(t, server, accountToken, iniciarAutorizacaoDe(t, server, pub.clienteBom(), escoposDoAgente))
	status, tokens := trocarCodigoDe(t, server, pub.clienteBom(), retorno.Get("code"))

	require.Equal(t, http.StatusOK, status, tokens)
	assert.NotEmpty(t, tokens["access_token"])
	assert.Equal(t, escoposDoAgente, tokens["scope"])
	require.NotEmpty(t, iniciarAutorizacaoDe(t, server, pub.clienteBom(), escoposDoAgente))
	assert.Equal(t, int32(1), pub.buscasBom.Load(), "dentro do TTL o documento vem do cache")
}

func TestIntegration_OAuthCIMD_DocumentoCom500_RecusaSemDetalheELembraAFalha(t *testing.T) {
	server, pub := setupCIMDIntegrationServer(t)

	for tentativa := 1; tentativa <= 2; tentativa++ {
		resp := pedirAutorizacao(t, server, pub.clienteQuebrado(), escoposDoAgente)
		corpo, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "tentativa %d", tentativa)
		assert.Empty(t, resp.Header.Get("Location"), "tentativa %d", tentativa)
		assert.Contains(t, string(corpo), `"error":"invalid_request"`, "tentativa %d", tentativa)
		assert.NotContains(t, string(corpo), "status 500", "tentativa %d", tentativa)
		assert.NotContains(t, string(corpo), "falha interna do host", "tentativa %d", tentativa)
	}
	assert.Equal(t, int32(1), pub.buscasQuebrado.Load(), "o cache negativo evita a segunda busca")
}
