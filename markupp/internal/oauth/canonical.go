package oauth

import (
	"net/url"
	"strings"
)

// CanonicalURL devolve a URL com esquema e host em minúsculas e sem barra final, a
// forma usada para comparar issuer, painel e resources.
func CanonicalURL(parsed *url.URL) string {
	canonical := *parsed
	canonical.Host = strings.ToLower(canonical.Host)
	canonical.Path = strings.TrimRight(canonical.Path, "/")
	canonical.RawPath = strings.TrimRight(canonical.RawPath, "/")
	return canonical.String()
}
