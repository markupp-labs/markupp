package api

import (
	"net/http"
	"net/netip"
	"strings"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// maxUserAgentBytes limita o user agent guardado na sessão e no histórico.
const maxUserAgentBytes = 512

// recordOrigin guarda no contexto o IP do cliente, com a regra de proxy confiável do
// limitador, e o user agent, para a sessão e a auditoria (ADR-0061).
func recordOrigin(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := auth.Origin{IP: originIP(r, trusted), UserAgent: cleanUserAgent(r.UserAgent())}
			next.ServeHTTP(w, r.WithContext(auth.WithOrigin(r.Context(), origin)))
		})
	}
}

func originIP(r *http.Request, trusted []netip.Prefix) string {
	addr, ok := clientAddr(r, trusted)
	if !ok {
		return connectionHost(r.RemoteAddr)
	}
	return addr.WithZone("").String()
}

// cleanUserAgent corta o header no limite e tira o UTF-8 inválido, que o Postgres recusa.
func cleanUserAgent(userAgent string) string {
	if len(userAgent) > maxUserAgentBytes {
		userAgent = userAgent[:maxUserAgentBytes]
	}
	return strings.ToValidUTF8(userAgent, "")
}
