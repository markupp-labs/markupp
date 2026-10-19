package cimd_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth/cimd"
)

func TestPublicOnly_FaixasNaoPublicas_Recusa(t *testing.T) {
	bloqueados := map[string]string{
		"loopback v4":         "127.0.0.1",
		"loopback v4 faixa":   "127.10.20.30",
		"loopback v6":         "::1",
		"rfc1918 10/8":        "10.1.2.3",
		"rfc1918 172.16/12":   "172.31.255.254",
		"rfc1918 192.168/16":  "192.168.0.10",
		"ula fc00::/7":        "fd12:3456::1",
		"link-local v4":       "169.254.169.254",
		"link-local v6":       "fe80::1",
		"unspecified v4":      "0.0.0.0",
		"unspecified v6":      "::",
		"multicast v4":        "224.0.0.1",
		"multicast v6":        "ff02::1",
		"cgnat inicio":        "100.64.0.1",
		"cgnat fim":           "100.127.255.254",
		"v4 mapeado em v6":    "::ffff:10.0.0.1",
		"loopback mapeado v6": "::ffff:127.0.0.1",
		"nat64 de loopback":   "64:ff9b::7f00:1",
		"nat64 de publico":    "64:ff9b::808:808",
		"6to4 de privado":     "2002:c0a8:1::1",
		"6to4 de publico":     "2002:808:808::1",
		"nat64 local":         "64:ff9b:1::a00:1",
		"v4 compativel":       "::a00:1",
		"v4 compativel pub":   "::808:808",
		"teredo":              "2001:0:4136:e378:8000:63bf:3fff:fdd2",
	}
	for nome, ip := range bloqueados {
		t.Run(nome, func(t *testing.T) {
			err := cimd.PublicOnly(netip.MustParseAddr(ip))

			require.ErrorIs(t, err, cimd.ErrBlockedAddress)
			assert.Contains(t, err.Error(), netip.MustParseAddr(ip).Unmap().String())
		})
	}
}

func TestPublicOnly_IPsPublicos_Permite(t *testing.T) {
	publicos := []string{"8.8.8.8", "100.128.0.1", "172.32.0.1", "2001:4860:4860::8888", "64:ff9c::1", "64:ff9b:2::1", "2001:1::1", "2003::1"}
	for _, ip := range publicos {
		t.Run(ip, func(t *testing.T) {
			assert.NoError(t, cimd.PublicOnly(netip.MustParseAddr(ip)))
		})
	}
}
