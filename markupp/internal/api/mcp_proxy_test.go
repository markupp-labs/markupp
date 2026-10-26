package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
)

// fakeMCPUpstream faz o papel do container do servidor MCP: responde com o caminho, o
// método e o Authorization que recebeu.
type fakeMCPUpstream struct {
	server *httptest.Server
}

func newFakeMCPUpstream(t *testing.T) *fakeMCPUpstream {
	t.Helper()
	upstream := &fakeMCPUpstream{}
	upstream.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Recebido", r.Method+" "+r.URL.RequestURI()+" "+r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(upstream.server.Close)
	return upstream
}

func routerWithMCP(upstreamURL string) http.Handler {
	return api.NewRouter(api.RouterDeps{Tokens: &fakeTokenValidator{}, MCPUpstreamURL: upstreamURL})
}

func TestMCPProxy_EncaminhaOEndpointComOTokenDoAgente(t *testing.T) {
	upstream := newFakeMCPUpstream(t)
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer token-do-agente")
	rec := httptest.NewRecorder()

	routerWithMCP(upstream.server.URL).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "POST /mcp Bearer token-do-agente", rec.Header().Get("X-Recebido"))
}

func TestMCPProxy_EncaminhaAMetadataDoRecursoProtegido(t *testing.T) {
	upstream := newFakeMCPUpstream(t)
	rec := httptest.NewRecorder()

	routerWithMCP(upstream.server.URL).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil))

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "GET /.well-known/oauth-protected-resource/mcp", rec.Header().Get("X-Recebido"))
}

func TestMCPProxy_SemUpstream_NaoMontaARota(t *testing.T) {
	rec := httptest.NewRecorder()

	routerWithMCP("").ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMCPProxy_UpstreamFora_Responde502(t *testing.T) {
	upstream := newFakeMCPUpstream(t)
	upstream.server.Close()
	rec := httptest.NewRecorder()

	routerWithMCP(upstream.server.URL).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestMCPProxy_EncaminhaOEndpointPublicoSemToken(t *testing.T) {
	upstream := newFakeMCPUpstream(t)
	rec := httptest.NewRecorder()

	routerWithMCP(upstream.server.URL).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp/public", strings.NewReader(`{}`)))

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "POST /mcp/public ", rec.Header().Get("X-Recebido"))
}
