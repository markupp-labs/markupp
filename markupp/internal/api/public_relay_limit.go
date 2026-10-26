package api

import (
	"net/http"
	"net/netip"
	"strings"
)

// RelayAuthenticator confere a credencial HTTP Basic de um resource, como o servidor
// MCP, que chama a leitura pública em nome de um agente (ADR-0051).
type RelayAuthenticator interface {
	AuthenticatesResource(resourceID, secret string) bool
}

// publicClientIP é o IP que conta no limite da leitura pública. O pedido do relay
// autenticado conta pelo agente que ele repassa; o resto, pelo clientIP de sempre.
func (l *AccountRateLimiter) publicClientIP(r *http.Request, relay RelayAuthenticator) string {
	if !isAuthenticatedRelay(r, relay) {
		return l.clientIP(r)
	}
	return l.relayedClientIP(r)
}

// isAuthenticatedRelay diz se o pedido traz o HTTP Basic de um resource configurado.
func isAuthenticatedRelay(r *http.Request, relay RelayAuthenticator) bool {
	if relay == nil {
		return false
	}
	resourceID, secret, protoErr := resourceCredentials(r)
	return protoErr == nil && relay.AuthenticatesResource(resourceID, secret)
}

// relayedClientIP trata o último salto do X-Forwarded-For, que o Go escreveu ao
// encaminhar /mcp/public, como a conexão que o agente abriu, e segue dali a regra do
// clientIP. Sem esse salto legível, vale a conexão do relay.
func (l *AccountRateLimiter) relayedClientIP(r *http.Request) string {
	hops := forwardedHops(r)
	last := len(hops) - 1
	agent, err := netip.ParseAddr(strings.TrimSpace(hops[last]))
	if err != nil {
		return l.clientIP(r)
	}
	return l.keyBehind(agent.Unmap(), hops[:last])
}
