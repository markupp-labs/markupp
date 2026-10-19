package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/benchfixture"
)

// BenchmarkNoteGraph mede as leituras do grafo sobre o cofre sintético de 10 mil
// notas indexado, do roteador ao banco, com a resolução por título na leitura.
func BenchmarkNoteGraph(b *testing.B) {
	router := roteadorDoCofreSintetico(b, benchfixture.GenerateVault(benchfixture.VaultSeed, benchfixture.VaultSize))
	rotas := map[string]string{
		"links":            "/v1/notes/cofre-bench-00042/links",
		"backlinks":        "/v1/notes/cofre-bench-00042/backlinks",
		"mentions":         "/v1/notes/cofre-bench-00042/mentions",
		"backmentions":     "/v1/notes/cofre-bench-00042/backmentions",
		"suggestions":      "/v1/notes/cofre-bench-00042/suggestions",
		"note":             "/v1/notes/cofre-bench-00042",
		"orphans":          "/v1/vaults/cofre-bench/notes?orphan=true",
		"by_pagerank":      "/v1/vaults/cofre-bench/notes?sort=pagerank",
		"vault_links":      "/v1/vaults/cofre-bench/links",
		"unresolved_links": "/v1/vaults/cofre-bench/links?status=unresolved",
		"communities":      "/v1/vaults/cofre-bench/communities",
	}
	maior := maiorComunidade(b, router)
	rotas["community"] = fmt.Sprintf("/v1/vaults/cofre-bench/communities/%d", maior)
	rotas["by_community"] = fmt.Sprintf("/v1/vaults/cofre-bench/notes?community=%d&sort=pagerank", maior)
	benchRotas(b, router, rotas)
}

// BenchmarkNoteGraph_Uniforme mede menção e sugestão no cofre uniforme, o pior caso:
// poucas palavras repetidas por todo o cofre e ligações espalhadas sem hub.
func BenchmarkNoteGraph_Uniforme(b *testing.B) {
	router := roteadorDoCofreSintetico(b, benchfixture.GenerateUniformVault(benchfixture.VaultSeed, benchfixture.VaultSize))
	benchRotas(b, router, map[string]string{
		"mentions":     "/v1/notes/cofre-bench-00042/mentions",
		"backmentions": "/v1/notes/cofre-bench-00042/backmentions",
		"suggestions":  "/v1/notes/cofre-bench-00042/suggestions",
	})
}

func benchRotas(b *testing.B, router http.Handler, rotas map[string]string) {
	for nome, rota := range rotas {
		b.Run(nome, func(b *testing.B) { benchGet(b, router, rota) })
	}
}

// maiorComunidade lê o id da maior comunidade do cofre sintético.
func maiorComunidade(b *testing.B, router http.Handler) int64 {
	b.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/vaults/cofre-bench/communities?limit=1", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var comunidades []struct {
		ID int64 `json:"id"`
	}
	require.NoError(b, json.NewDecoder(rec.Body).Decode(&comunidades), rec.Body.String())
	require.NotEmpty(b, comunidades, "o segundo estagio agrupa o cofre sintetico")
	return comunidades[0].ID
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
