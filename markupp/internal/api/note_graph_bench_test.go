package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// BenchmarkNoteGraph mede as leituras do grafo sobre o cofre sintético de 10 mil
// notas indexado, do roteador ao banco, com a resolução por título na leitura.
func BenchmarkNoteGraph(b *testing.B) {
	router := roteadorDoCofreSintetico(b)
	rotas := map[string]string{
		"backlinks":        "/notes/cofre-bench-00042/backlinks",
		"neighbors":        "/notes/cofre-bench-00042/neighbors",
		"mentions":         "/notes/cofre-bench-00042/mentions",
		"suggestions":      "/notes/cofre-bench-00042/suggestions",
		"note":             "/v1/notes/cofre-bench-00042",
		"orphans":          "/v1/vaults/cofre-bench/notes?orphan=true",
		"by_pagerank":      "/v1/vaults/cofre-bench/notes?sort=pagerank",
		"unresolved_links": "/vaults/cofre-bench/unresolved-links",
		"clusters":         "/vaults/cofre-bench/clusters",
	}
	for nome, rota := range rotas {
		b.Run(nome, func(b *testing.B) { benchGet(b, router, rota) })
	}
}

func benchGet(b *testing.B, router http.Handler, rota string) {
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, rota, nil)
		req.Header.Set("Authorization", "Bearer token-valido")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(b, http.StatusOK, rec.Code, "GET %s: %s", rota, rec.Body.String())
	}
}
