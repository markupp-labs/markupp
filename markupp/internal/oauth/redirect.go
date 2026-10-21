package oauth

import (
	"net/url"
	"strings"
)

// MatchRedirectURI diz se a redirect_uri pedida bate com alguma das registradas pelo
// cliente. A comparação é de string exata; a única exceção é o loopback por IP da
// RFC 8252 (http em 127.0.0.1 ou [::1]), em que a porta é ignorada. localhost por nome
// nunca ganha a exceção, e fragmento na pedida sempre recusa.
func MatchRedirectURI(registered []string, requested string) bool {
	if requested == "" || strings.Contains(requested, "#") {
		return false
	}
	for _, candidate := range registered {
		if candidate == requested || matchLoopbackIgnoringPort(candidate, requested) {
			return true
		}
	}
	return false
}

// matchLoopbackIgnoringPort compara as duas URIs de loopback depois de tirar a porta.
func matchLoopbackIgnoringPort(registered, requested string) bool {
	reg, ok := loopbackWithoutPort(registered)
	if !ok {
		return false
	}
	req, ok := loopbackWithoutPort(requested)
	return ok && reg == req
}

// loopbackWithoutPort devolve a URI sem a porta quando ela é http, sem credencial e
// com host 127.0.0.1 ou [::1] escrito literalmente; ok falso em qualquer outro caso.
// O resto da string (caminho e query) segue intacto, sem normalização.
func loopbackWithoutPort(raw string) (string, bool) {
	const scheme = "http://"
	if !strings.HasPrefix(raw, scheme) {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || !loopbackIP(parsed.Hostname()) {
		return "", false
	}
	rest := strings.TrimPrefix(raw, scheme+parsed.Host)
	return scheme + bracketIPv6(parsed.Hostname()) + rest, true
}

func loopbackIP(host string) bool {
	return host == "127.0.0.1" || host == "::1"
}

func bracketIPv6(host string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}
