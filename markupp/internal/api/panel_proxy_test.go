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
