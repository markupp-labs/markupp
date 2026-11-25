package api

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

var redeDoPainel = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}

// origemVista roda o pedido pelo middleware e devolve a origem que o handler recebeu.
func origemVista(r *http.Request) auth.Origin {
	var got auth.Origin
	recordOrigin(redeDoPainel)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = auth.OriginFrom(r.Context())
	})).ServeHTTP(httptest.NewRecorder(), r)
	return got
}

func pedidoDe(remoteAddr, forwardedFor, userAgent string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	r.RemoteAddr = remoteAddr
	r.Header.Set("User-Agent", userAgent)
	if forwardedFor != "" {
		r.Header.Set("X-Forwarded-For", forwardedFor)
	}
	return r
}

func TestRecordOrigin_ConexaoDireta_UsaIPDaConexaoEUserAgent(t *testing.T) {
	got := origemVista(pedidoDe("203.0.113.7:51000", "", "Firefox/140"))

	assert.Equal(t, auth.Origin{IP: "203.0.113.7", UserAgent: "Firefox/140"}, got)
}

func TestRecordOrigin_PainelConfiavel_UsaIPDoNavegador(t *testing.T) {
	got := origemVista(pedidoDe("10.0.0.5:51000", "198.51.100.9", "Chrome/150"))

	assert.Equal(t, "198.51.100.9", got.IP)
}

func TestRecordOrigin_ConexaoNaoConfiavel_IgnoraForwardedFor(t *testing.T) {
	got := origemVista(pedidoDe("203.0.113.7:51000", "198.51.100.9", "curl/8"))

	assert.Equal(t, "203.0.113.7", got.IP)
}

func TestRecordOrigin_IPv6_GuardaEnderecoInteiro(t *testing.T) {
	got := origemVista(pedidoDe("[2001:db8::1234]:51000", "", "Firefox/140"))

	assert.Equal(t, "2001:db8::1234", got.IP, "o limite agrupa por /64, a sessão mostra o endereço")
}

func TestRecordOrigin_UserAgentLongo_Corta(t *testing.T) {
	got := origemVista(pedidoDe("203.0.113.7:51000", "", strings.Repeat("a", 2000)))

	assert.Len(t, got.UserAgent, maxUserAgentBytes)
}
