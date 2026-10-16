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

var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

// PublicOnly é a política padrão: recusa loopback, privados (RFC 1918 e fc00::/7),
// link-local, unspecified, multicast e CGNAT (100.64.0.0/10).
func PublicOnly(ip netip.Addr) error {
	ip = ip.Unmap()
	if isPublicIP(ip) {
		return nil
	}
	return fmt.Errorf("%w: %s; esperado IP publico (fora de loopback, privado, link-local, unspecified, multicast e CGNAT)",
		ErrBlockedAddress, ip)
}

func isPublicIP(ip netip.Addr) bool {
	return ip.IsValid() &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsUnspecified() &&
		!ip.IsMulticast() &&
		!cgnatPrefix.Contains(ip)
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
