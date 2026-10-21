package cimd

import (
	"net/url"
	"strings"
)

// RedirectURIShape descreve a redirect_uri aceita, para a mensagem de erro de quem a
// recusa.
const RedirectURIShape = "https com host, http em 127.0.0.1 ou [::1], ou esquema privado (RFC 8252, secao 7.1) " +
	"fora de javascript, data, vbscript e file, sem fragmento e sem credenciais"

// forbiddenRedirectSchemes são os esquemas que executam código ou leem arquivo local; o
// vazio é a URL relativa. O url.Parse já devolve o esquema em minúsculas.
var forbiddenRedirectSchemes = map[string]bool{"": true, "javascript": true, "data": true, "vbscript": true, "file": true}

// RedirectURIProblem diz o que impede raw de ser redirect_uri de um cliente público, ou
// devolve vazio quando ela serve. Vale para o documento CIMD e para o registro dinâmico.
func RedirectURIProblem(raw string) string {
	parsed, err := url.Parse(raw)
	switch {
	case err != nil:
		return "nao e uma URL"
	case strings.Contains(raw, "#"):
		return "contem fragmento"
	case parsed.User != nil:
		return "contem credenciais"
	}
	return redirectSchemeProblem(parsed)
}

// redirectSchemeProblem aceita qualquer esquema privado, com ou sem ponto, fora dos
// proibidos. URL opaca de http/https sai sem host.
func redirectSchemeProblem(parsed *url.URL) string {
	switch parsed.Scheme {
	case "https":
		if parsed.Hostname() == "" {
			return "https sem host"
		}
	case "http":
		if host := parsed.Hostname(); host != "127.0.0.1" && host != "::1" {
			return "http fora de 127.0.0.1 e [::1]"
		}
	default:
		if forbiddenRedirectSchemes[parsed.Scheme] {
			return "esquema proibido ou URL relativa"
		}
	}
	return ""
}
