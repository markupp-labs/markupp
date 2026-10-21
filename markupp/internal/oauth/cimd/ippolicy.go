package cimd

import (
	"errors"
	"fmt"
	"net/netip"
	"syscall"
)

// ErrBlockedAddress indica que o host resolveu para um IP fora da internet pública.
var ErrBlockedAddress = errors.New("endereco bloqueado")

// IPPolicy decide, depois da resolução DNS, se a conexão a um IP é permitida.
// Devolve nil para permitir ou um erro que embrulha ErrBlockedAddress.
type IPPolicy func(ip netip.Addr) error

// blockedPrefixes cobre o que o netip não classifica: CGNAT, a faixa de benchmark
// (198.18.0.0/15), a reservada 240.0.0.0/4 (com o broadcast), o site-local IPv6
// (fec0::/10) e os prefixos de tradução NAT64 (o bem conhecido e o local),
// IPv4-compatível, 6to4 e Teredo, que embutem um IPv4 qualquer (inclusive interno) no
// endereço IPv6.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("fec0::/10"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("2001::/32"),
}

// PublicOnly é a política padrão: recusa loopback, privados (RFC 1918 e fc00::/7),
// link-local, unspecified, multicast, CGNAT (100.64.0.0/10), benchmark (198.18.0.0/15),
// reservado (240.0.0.0/4), site-local (fec0::/10), NAT64 (64:ff9b::/96 e
// 64:ff9b:1::/48), IPv4-compatível (::/96), 6to4 (2002::/16) e Teredo (2001::/32).
func PublicOnly(ip netip.Addr) error {
	ip = ip.Unmap()
	if isPublicIP(ip) {
		return nil
	}
	return fmt.Errorf("%w: %s; esperado IP publico (fora de loopback, privado, link-local, unspecified, "+
		"multicast, CGNAT, benchmark, reservado, site-local, NAT64, IPv4-compativel, 6to4 e Teredo)", ErrBlockedAddress, ip)
}

func isPublicIP(ip netip.Addr) bool {
	return ip.IsValid() &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsUnspecified() &&
		!ip.IsMulticast() &&
		!inBlockedPrefix(ip)
}

func inBlockedPrefix(ip netip.Addr) bool {
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// dialControl aplica a política no Control do Dialer, que recebe o IP já resolvido;
// assim um DNS que aponta para a rede interna não escapa da checagem.
func (p IPPolicy) dialControl(_, address string, _ syscall.RawConn) error {
	addrPort, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: %q; esperado ip:porta", ErrBlockedAddress, address)
	}
	return p(addrPort.Addr())
}
