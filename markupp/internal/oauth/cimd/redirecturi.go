package cimd

import (
	"net/url"
	"strings"
)

// RedirectURIShape descreve a redirect_uri aceita, para a mensagem de erro de quem a
// recusa.
const RedirectURIShape = "https with a host, http on 127.0.0.1, [::1] or localhost, or a private-use scheme (RFC 8252, section 7.1) " +
	"other than javascript, data, vbscript and file, without fragment or credentials"

// forbiddenRedirectSchemes são os esquemas que executam código ou leem arquivo local; o
// vazio é a URL relativa. O url.Parse já devolve o esquema em minúsculas.
var forbiddenRedirectSchemes = map[string]bool{"": true, "javascript": true, "data": true, "vbscript": true, "file": true}

// RedirectURIProblem diz o que impede raw de ser redirect_uri de um cliente público, ou
// devolve vazio quando ela serve. Vale para o documento CIMD e para o registro dinâmico.
func RedirectURIProblem(raw string) string {
	parsed, err := url.Parse(raw)
	switch {
	case err != nil:
		return "is not a URL"
	case strings.Contains(raw, "#"):
		return "contains a fragment"
	case parsed.User != nil:
		return "contains credentials"
	}
	return redirectSchemeProblem(parsed)
}

// LoopbackHost diz se o host é o loopback que a RFC 8252 aceita em http. localhost entra,
// embora a RFC prefira o IP: é o que clientes como o Claude Code registram.
func LoopbackHost(host string) bool {
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// redirectSchemeProblem aceita qualquer esquema privado, com ou sem ponto, fora dos
// proibidos. URL opaca de http/https sai sem host.
func redirectSchemeProblem(parsed *url.URL) string {
	switch parsed.Scheme {
	case "https":
		if parsed.Hostname() == "" {
			return "uses https without a host"
		}
	case "http":
		if !LoopbackHost(parsed.Hostname()) {
			return "uses http outside 127.0.0.1, [::1] and localhost"
		}
	default:
		if forbiddenRedirectSchemes[parsed.Scheme] {
			return "uses a forbidden scheme or is a relative URL"
		}
	}
	return ""
}
