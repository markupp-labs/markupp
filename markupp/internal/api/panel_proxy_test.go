package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
)

// newFakePanelUpstream faz o papel do container do painel: responde com o caminho e o
// Host que recebeu.
func newFakePanelUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Recebido", r.Method+" "+r.URL.RequestURI()+" "+r.Host)
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(server.Close)
	return server
}

func routerWithPanel(upstreamURL string) http.Handler {
	return api.NewRouter(api.RouterDeps{Tokens: &fakeTokenValidator{}, Probe: &fakeProbe{}, PanelUpstreamURL: upstreamURL})
}

func TestPanelProxy_EncaminhaARaizEOResto_ComOHostOriginal(t *testing.T) {
	upstream := newFakePanelUpstream(t)
	for _, path := range []string{"/", "/notes/1?mode=read", "/oauth/requests/r1", "/auth/github/callback"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = "notas.exemplo.com"
		rec := httptest.NewRecorder()

		routerWithPanel(upstream.URL).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusTeapot, rec.Code, path)
		assert.Equal(t, "GET "+path+" notas.exemplo.com", rec.Header().Get("X-Recebido"), path)
	}
}

// routerWithPanelAndOAuth monta o padrão do compose: painel na raiz da mesma porta e
// authorization server ligado.
func routerWithPanelAndOAuth(upstreamURL string) http.Handler {
	return api.NewRouter(api.RouterDeps{
		Tokens: &fakeTokenValidator{}, Probe: &fakeProbe{}, PanelUpstreamURL: upstreamURL,
		OAuth: &fakeOAuthService{}, OAuthMetadata: &api.OAuthMetadata{Issuer: issuerDeTeste, DynamicRegistration: true},
	})
}

func TestPanelProxy_ComOAuth_EncaminhaOConsentimentoAoPainel(t *testing.T) {
	upstream := newFakePanelUpstream(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/oauth/requests/r1"},
		{http.MethodPost, "/oauth/requests/r1/approve"},
		{http.MethodPost, "/oauth/requests/r1/deny"},
	} {
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Host = "notas.exemplo.com"
		rec := httptest.NewRecorder()

		routerWithPanelAndOAuth(upstream.URL).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusTeapot, rec.Code, route.path)
		assert.Equal(t, route.method+" "+route.path+" notas.exemplo.com", rec.Header().Get("X-Recebido"), route.path)
	}
}

func TestPanelProxy_ComOAuth_NaoTomaAsRotasDoProtocolo(t *testing.T) {
	upstream := newFakePanelUpstream(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/oauth/token"},
		{http.MethodPost, "/oauth/revoke"},
		{http.MethodPost, "/oauth/register"},
		{http.MethodGet, "/oauth/authorize"},
	} {
		rec := httptest.NewRecorder()

		routerWithPanelAndOAuth(upstream.URL).ServeHTTP(rec, httptest.NewRequest(route.method, route.path, nil))

		assert.Empty(t, rec.Header().Get("X-Recebido"), route.method+" "+route.path)
	}
}

func TestPanelProxy_NaoTomaAsRotasDaAPI(t *testing.T) {
	upstream := newFakePanelUpstream(t)
	for _, path := range []string{"/v1/healthz", "/v1/nao-existe"} {
		rec := httptest.NewRecorder()

		routerWithPanel(upstream.URL).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		assert.Empty(t, rec.Header().Get("X-Recebido"), path)
	}
}

func TestPanelProxy_SemUpstream_RaizResponde404(t *testing.T) {
	rec := httptest.NewRecorder()

	routerWithPanel("").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
