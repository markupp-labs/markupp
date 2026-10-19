package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/benchfixture"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// BenchmarkSearchNotes mede GET /notes/search sobre o cofre sintético de 10 mil
// notas, do roteador ao banco, como linha de base da busca léxica.
func BenchmarkSearchNotes(b *testing.B) {
	router := roteadorDoCofreSintetico(b)
	termos := map[string]string{
		"termo_frequente": "projeto",
		"termo_ausente":   "inexistente",
		"titulo":          "00042",
	}
	for nome, termo := range termos {
		b.Run(nome, func(b *testing.B) { benchBusca(b, router, termo) })
	}
}

func benchBusca(b *testing.B, router http.Handler, termo string) {
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/notes/search?query="+url.QueryEscape(termo), nil)
		req.Header.Set("Authorization", "Bearer token-valido")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(b, http.StatusOK, rec.Code, "busca por %q: %s", termo, rec.Body.String())
	}
}

// indexaCofreSintetico esvazia a fila do indexador, porque a busca lê só o índice.
func indexaCofreSintetico(b *testing.B, pool *pgxpool.Pool) {
	b.Helper()
	indexer, err := storage.NewPostgresVaultIndexer(context.Background(), pool, "simple")
	require.NoError(b, err)
	for {
		found, err := indexer.IndexNextVault(context.Background(), indexing.Index)
		require.NoError(b, err)
		if !found {
			return
		}
	}
}

// roteadorDoCofreSintetico monta o roteador real, com busca e grafo, sobre um banco
// com o cofre sintético inteiro indexado e analisado, visto por um membro dele. Sem
// ANALYZE o planejador não conhece a carga recém-feita e escolhe laços aninhados que
// o autovacuum de uma instalação real já teria evitado.
func roteadorDoCofreSintetico(b *testing.B) http.Handler {
	b.Helper()
	pool := storagetest.EmptyPool(b)
	require.NoError(b, storage.Migrate(context.Background(), pool))
	ator := identity.Actor{UserID: "u-bench", TenantID: "t-bench", Role: "member"}
	storagetest.SeedVault(b, pool, ator, "cofre-bench", benchfixture.GenerateVault(benchfixture.VaultSeed, benchfixture.VaultSize))
	indexaCofreSintetico(b, pool)
	_, err := storage.RequeueNotes(context.Background(), pool, "cofre-bench")
	require.NoError(b, err, "a primeira passada escolhe os termos sem df; a segunda, com o df do segundo estagio")
	indexaCofreSintetico(b, pool)
	_, err = pool.Exec(context.Background(), "ANALYZE")
	require.NoError(b, err, "estatisticas da carga, como o autovacuum deixaria")
	svc := notes.NewService(storage.NewPostgresNotesRepository(pool, "simple"), integrationMaxNoteSize)
	return api.NewRouter(api.RouterDeps{
		Notes:   svc,
		Graph:   storage.NewPostgresNoteGraph(pool, "simple"),
		Tenants: &fakeTenantResolver{membership: auth.Membership{TenantID: ator.TenantID, Role: ator.Role}},
		Tokens:  &fakeTokenValidator{claims: auth.Claims{Subject: ator.UserID}},
		Probe:   pool,
	})
}
