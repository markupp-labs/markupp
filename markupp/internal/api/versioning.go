package api

import "net/http"

const currentVersionPrefix = "/v1"

// deprecatedSince é a data em que as rotas sem prefixo ficaram obsoletas, no formato
// de data do cabeçalho Deprecation (RFC 9745): 2026-10-16T00:00:00Z.
const deprecatedSince = "@1792108800"

// markDeprecated marca a resposta como obsoleta e aponta o mesmo caminho sob /v1.
func markDeprecated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Deprecation", deprecatedSince)
		successor := currentVersionPrefix + r.URL.EscapedPath()
		w.Header().Set("Link", "<"+successor+`>; rel="successor-version"`)
		next.ServeHTTP(w, r)
	})
}
