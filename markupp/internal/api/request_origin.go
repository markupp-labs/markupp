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
// limitador, e o user agent, para a sessão e a auditoria (ADR-0061). O relay autenticado,
// como o servidor MCP na troca de token, repassa o IP e o user agent do agente.
func recordOrigin(trusted []netip.Prefix, relay RelayAuthenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := auth.Origin{IP: originIP(r, trusted, relay), UserAgent: cleanUserAgent(r.UserAgent())}
			next.ServeHTTP(w, r.WithContext(auth.WithOrigin(r.Context(), origin)))
		})
	}
}

func originIP(r *http.Request, trusted []netip.Prefix, relay RelayAuthenticator) string {
	addr, ok := clientAddr(r, trusted)
	if isAuthenticatedRelay(r, relay) {
		addr, ok = relayedClientAddr(r, trusted)
	}
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
