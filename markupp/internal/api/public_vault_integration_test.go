package api_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// servidorDePublicacao sobe a API sobre um banco novo, com o ator no papel role do
// tenant t1, membro do cofre "va", que tem a nota "n1".
func servidorDePublicacao(t *testing.T, role string) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	pool := storagetest.EmptyPool(t)
	require.NoError(t, storage.Migrate(context.Background(), pool))
	ctx := storagetest.SeedMember(t, pool, identity.Actor{UserID: "u1", TenantID: "t1", Role: role}, "va")
	agora := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	storagetest.SaveNote(ctx, t, storage.NewPostgresNotesRepository(pool, "simple"),
		notes.Note{ID: "n1", VaultID: "va", Path: "inicio.md", Content: "# Comece aqui", CreatedAt: agora, UpdatedAt: agora})
	router := api.NewRouter(api.RouterDeps{
		Tenants:      &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: role}},
		Tokens:       &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}},
		Publications: publication.NewService(storage.NewPostgresVaultPublications(pool), time.Now),
		PublicVaults: storage.NewPostgresPublicVaults(pool, "simple"),
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server, pool
}

// respostaDaAPI é o status e o corpo já lido de uma chamada.
type respostaDaAPI struct {
	StatusCode int
	Body       *bytes.Reader
}

func chamaAPI(t *testing.T, method, url, body string, autenticado bool) respostaDaAPI {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	require.NoError(t, err)
	if autenticado {
		req.Header.Set("Authorization", "Bearer token-valido")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	lido, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return respostaDaAPI{StatusCode: resp.StatusCode, Body: bytes.NewReader(lido)}
}

func TestIntegracao_AdminPublica_LeSemContaEDespublica(t *testing.T) {
	server, pool := servidorDePublicacao(t, "admin")

	publicou := chamaAPI(t, http.MethodPut, server.URL+"/v1/vaults/va/publication", `{"slug":"docs","title":"Docs"}`, true)
	require.Equal(t, http.StatusOK, publicou.StatusCode)

	nota := chamaAPI(t, http.MethodGet, server.URL+"/v1/public/vaults/docs/notes/n1", "", false)
	require.Equal(t, http.StatusOK, nota.StatusCode)
	assert.Equal(t, "# Comece aqui", decodeJSON(t, nota.Body)["content"])

	despublicou := chamaAPI(t, http.MethodDelete, server.URL+"/v1/vaults/va/publication", "", true)
	require.Equal(t, http.StatusNoContent, despublicou.StatusCode)

	depois := chamaAPI(t, http.MethodGet, server.URL+"/v1/public/vaults/docs/notes/n1", "", false)
	assert.Equal(t, http.StatusNotFound, depois.StatusCode)
	var eventos int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT count(*) FROM audit_events WHERE action IN ('vaults.publish', 'vaults.unpublish') AND target = 'va'").Scan(&eventos))
	assert.Equal(t, 2, eventos)
}

func TestIntegracao_CofreNaoPublicado_Responde404SemConta(t *testing.T) {
	server, _ := servidorDePublicacao(t, "admin")

	resp := chamaAPI(t, http.MethodGet, server.URL+"/v1/public/vaults/va", "", false)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "nem o id do cofre abre a leitura sem publicacao")
}

func TestIntegracao_MembroNaoPublica(t *testing.T) {
	server, pool := servidorDePublicacao(t, "member")

	resp := chamaAPI(t, http.MethodPut, server.URL+"/v1/vaults/va/publication", `{"slug":"docs"}`, true)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	var publicados int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM vault_publications").Scan(&publicados))
	assert.Zero(t, publicados)
}

func TestIntegracao_SlugDeOutroTenant_Responde409(t *testing.T) {
	server, pool := servidorDePublicacao(t, "owner")
	storagetest.SeedMember(t, pool, identity.Actor{UserID: "u2", TenantID: "t2", Role: "owner"}, "vb")
	_, err := pool.Exec(context.Background(), `INSERT INTO vault_publications (vault_id, tenant_id, slug, title, published_by, published_at)
		VALUES ('vb', 't2', 'docs', 'Outro', 'u2', now())`)
	require.NoError(t, err)

	resp := chamaAPI(t, http.MethodPut, server.URL+"/v1/vaults/va/publication", `{"slug":"docs"}`, true)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}
