package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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
// tenant t1, membro do cofre "va", que tem a nota "n1" na pasta docs.
func servidorDePublicacao(t *testing.T, role string) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	pool := storagetest.EmptyPool(t)
	require.NoError(t, storage.Migrate(context.Background(), pool))
	ctx := storagetest.SeedMember(t, pool, identity.Actor{UserID: "u1", TenantID: "t1", Role: role}, "va")
	agora := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	storagetest.SaveNote(ctx, t, storage.NewPostgresNotesRepository(pool, "simple"),
		notes.Note{ID: "n1", VaultID: "va", Path: "docs/Comece aqui.md", Content: "# Comece aqui", CreatedAt: agora, UpdatedAt: agora})
	router := api.NewRouter(api.RouterDeps{
		Tenants:      &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: role}},
		Tokens:       &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}},
		Publications: publication.NewService(storage.NewPostgresPublications(pool), time.Now, uuid.NewString),
		PublicNotes:  storage.NewPostgresPublicNotes(pool, "simple"),
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server, pool
}

// respostaDaAPI é o status e o corpo já decodificado de uma chamada.
type respostaDaAPI struct {
	status int
	corpo  map[string]any
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
	corpo := map[string]any{}
	if len(lido) > 0 {
		require.NoError(t, json.NewDecoder(bytes.NewReader(lido)).Decode(&corpo), string(lido))
	}
	return respostaDaAPI{status: resp.StatusCode, corpo: corpo}
}

func TestIntegracao_AdminPublicaPasta_LeSemContaEDespublica(t *testing.T) {
	server, pool := servidorDePublicacao(t, "admin")

	publicou := chamaAPI(t, http.MethodPost, server.URL+"/v1/vaults/va/publications", `{"target":{"kind":"folder","path":"docs"},"confirm":"docs"}`, true)
	require.Equal(t, http.StatusCreated, publicou.status, publicou.corpo)
	nota := chamaAPI(t, http.MethodGet, server.URL+"/v1/notes/n1", "", true)
	idPublico, ok := nota.corpo["public_id"].(string)
	require.True(t, ok, "nota publica sem public_id: %v", nota.corpo)

	lida := chamaAPI(t, http.MethodGet, server.URL+"/v1/public/notes/"+idPublico, "", false)
	require.Equal(t, http.StatusOK, lida.status)
	assert.Equal(t, "# Comece aqui", lida.corpo["content"])
	assert.NotContains(t, lida.corpo, "id", "o id interno nao sai")

	despublicou := chamaAPI(t, http.MethodDelete, server.URL+"/v1/publications/"+publicou.corpo["id"].(string), "", true)
	require.Equal(t, http.StatusNoContent, despublicou.status)
	depois := chamaAPI(t, http.MethodGet, server.URL+"/v1/public/notes/"+idPublico, "", false)
	assert.Equal(t, http.StatusNotFound, depois.status)
	var eventos int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT count(*) FROM audit_events WHERE action IN ('publications.publish', 'publications.unpublish')").Scan(&eventos))
	assert.Equal(t, 2, eventos)
}

func TestIntegracao_MembroPublicaANotaMasNaoAPasta(t *testing.T) {
	server, _ := servidorDePublicacao(t, "member")

	pasta := chamaAPI(t, http.MethodPost, server.URL+"/v1/vaults/va/publications", `{"target":{"kind":"folder","path":"docs"},"confirm":"docs"}`, true)
	nota := chamaAPI(t, http.MethodPost, server.URL+"/v1/vaults/va/publications", `{"target":{"kind":"note","note_id":"n1"},"confirm":"Comece aqui"}`, true)

	assert.Equal(t, http.StatusForbidden, pasta.status)
	assert.Equal(t, "publish_forbidden", pasta.corpo["error"])
	require.Equal(t, http.StatusCreated, nota.status, nota.corpo)
	assert.Equal(t, "comece-aqui", nota.corpo["title_slug"])
}

func TestIntegracao_ConfirmacaoErrada_422SemPublicar(t *testing.T) {
	server, pool := servidorDePublicacao(t, "owner")

	resp := chamaAPI(t, http.MethodPost, server.URL+"/v1/vaults/va/publications", `{"target":{"kind":"vault"},"confirm":"outro"}`, true)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.status)
	assert.Contains(t, resp.corpo["message"], `"va"`)
	var publicadas int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM publications").Scan(&publicadas))
	assert.Zero(t, publicadas)
}
