package api

import (
	"net/http/httputil"
	"net/netip"
)

// setForwardedHeaders escreve os X-Forwarded-* do pedido encaminhado. O Rewrite chega
// sem o X-Forwarded-For de entrada; ele só volta quando a conexão vem de proxy
// confiável, senão o cliente escolheria o IP que o painel e o MCP veem.
func setForwardedHeaders(pr *httputil.ProxyRequest, trusted []netip.Prefix) {
	if isTrustedPeer(pr.In.RemoteAddr, trusted) {
		pr.Out.Header["X-Forwarded-For"] = pr.In.Header["X-Forwarded-For"]
	}
	pr.SetXForwarded()
}

// isTrustedPeer diz se a conexão remoteAddr vem de uma das redes trusted.
func isTrustedPeer(remoteAddr string, trusted []netip.Prefix) bool {
	addr, err := netip.ParseAddr(connectionHost(remoteAddr))
	return err == nil && containsAddr(trusted, addr.Unmap())
}
