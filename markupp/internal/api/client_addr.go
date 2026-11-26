package api

import (
	"net/http"
	"net/netip"
	"strings"
)

// clientAddr usa o IP da conexão e, quando ela vem de proxy confiável, o primeiro IP
// não confiável do X-Forwarded-For lido da direita, porque o que está à esquerda
// dele o cliente escreve como quiser. Header vazio ou malformado fica com a conexão.
// Falso quando nem a conexão tem IP legível.
func clientAddr(r *http.Request, trusted []netip.Prefix) (netip.Addr, bool) {
	remote, err := netip.ParseAddr(connectionHost(r.RemoteAddr))
	if err != nil {
		return netip.Addr{}, false
	}
	return addrBehind(remote.Unmap(), forwardedHops(r), trusted), true
}

// addrBehind é o cliente atrás de remote, a conexão vista, com os saltos do
// X-Forwarded-For que chegaram nela.
func addrBehind(remote netip.Addr, hops []string, trusted []netip.Prefix) netip.Addr {
	if forwarded, ok := forwardedClient(hops, remote, trusted); ok {
		return forwarded
	}
	return remote
}

// forwardedHops junta os headers X-Forwarded-For numa lista de saltos, sempre com ao
// menos um item, vazio quando o header falta.
func forwardedHops(r *http.Request) []string {
	return strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
}

// forwardedClient percorre o X-Forwarded-For da direita para a esquerda enquanto o
// salto anterior é proxy confiável. Entrada que não é IP invalida o header.
func forwardedClient(hops []string, remote netip.Addr, trusted []netip.Prefix) (netip.Addr, bool) {
	for i := len(hops) - 1; i >= 0 && containsAddr(trusted, remote); i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return netip.Addr{}, false
		}
		remote = hop.Unmap()
	}
	return remote, !containsAddr(trusted, remote)
}

// connectionHost tira a porta do RemoteAddr; sem porta legível, devolve o valor como veio.
func connectionHost(remoteAddr string) string {
	if addrPort, err := netip.ParseAddrPort(remoteAddr); err == nil {
		return addrPort.Addr().String()
	}
	return remoteAddr
}
