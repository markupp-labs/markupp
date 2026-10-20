package oauth

import (
	"net/url"
	"strings"
)

// defaultPorts é a porta implícita de cada esquema, que a forma canônica remove.
var defaultPorts = map[string]string{"https": "443", "http": "80"}

// CanonicalURL devolve a URL com esquema e host em minúsculas, sem a porta padrão do
// esquema e sem barra final, a forma usada para comparar issuer, painel e resources.
func CanonicalURL(parsed *url.URL) string {
	canonical := *parsed
	canonical.Host = withoutDefaultPort(strings.ToLower(canonical.Host), canonical.Scheme)
	canonical.Path = strings.TrimRight(canonical.Path, "/")
	canonical.RawPath = strings.TrimRight(canonical.RawPath, "/")
	return canonical.String()
}

// withoutDefaultPort tira de host a porta padrão do esquema, para que https://h:443 e
// https://h virem a mesma URL. Esquema sem porta padrão conhecida deixa o host intacto.
func withoutDefaultPort(host, scheme string) string {
	port, known := defaultPorts[strings.ToLower(scheme)]
	if !known {
		return host
	}
	return strings.TrimSuffix(host, ":"+port)
}
