package api_test

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

const integrationMaxNoteSize = 50 * 1024 * 1024

func setupIntegrationServer(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, storage.Migrate(db))

	repo := storage.NewSqliteNotesRepository(db)
	svc := notes.NewService(repo, integrationMaxNoteSize)
	router := api.NewRouter(svc, db, nil)

	server := httptest.NewServer(router)
	t.Cleanup(func() {
		server.Close()
		_ = db.Close()
	})
	return server, db
}

func postNotes(t *testing.T, baseURL, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(baseURL+"/notes", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	return resp
}

func putNote(t *testing.T, baseURL, id, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, baseURL+"/notes/"+id, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func deleteNote(t *testing.T, baseURL, id string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/notes/"+id, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeJSON(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.NewDecoder(body).Decode(&result))
	return result
}

func TestIntegration_CriarNota_FluxoCompleto(t *testing.T) {
	server, db := setupIntegrationServer(t)

	resp := postNotes(t, server.URL, `{"path":"integracao.md","content":"# teste"}`)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	body := decodeJSON(t, resp.Body)
	id, _ := body["id"].(string)
	assert.NotEmpty(t, id)
	assert.Equal(t, "integracao.md", body["path"])
	assert.Equal(t, "# teste", body["content"])

	var dbPath, dbContent string
	err := db.QueryRow("SELECT path, content FROM notes WHERE id = ?", id).Scan(&dbPath, &dbContent)
	require.NoError(t, err)
	assert.Equal(t, "integracao.md", dbPath)
	assert.Equal(t, "# teste", dbContent)
}

func TestIntegration_AtualizarNota_FluxoCompleto(t *testing.T) {
	server, db := setupIntegrationServer(t)

	created := postNotes(t, server.URL, `{"path":"orig.md","content":"v1"}`)
	require.Equal(t, http.StatusCreated, created.StatusCode)
	createdBody := decodeJSON(t, created.Body)
	_ = created.Body.Close()
	id, _ := createdBody["id"].(string)
	require.NotEmpty(t, id)
	updatedAtStr, _ := createdBody["updated_at"].(string)

	updated := putNote(t, server.URL, id, `{"path":"renomeada.md","content":"v2","lastModifiedAt":"`+updatedAtStr+`","force":false}`)
	defer func() { _ = updated.Body.Close() }()
	require.Equal(t, http.StatusOK, updated.StatusCode)

	body := decodeJSON(t, updated.Body)
	assert.Equal(t, id, body["id"])
	assert.Equal(t, "renomeada.md", body["path"])
	assert.Equal(t, "v2", body["content"])

	var dbPath, dbContent string
	require.NoError(t, db.QueryRow("SELECT path, content FROM notes WHERE id = ?", id).Scan(&dbPath, &dbContent))
	assert.Equal(t, "renomeada.md", dbPath)
	assert.Equal(t, "v2", dbContent)
}

func TestIntegration_AtualizarNotaInexistente_Retorna404(t *testing.T) {
	server, _ := setupIntegrationServer(t)
	now := "2026-05-29T10:00:00Z"

	resp := putNote(t, server.URL, "fantasma", `{"path":"x.md","content":"y","lastModifiedAt":"`+now+`","force":false}`)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeJSON(t, resp.Body)
	assert.Equal(t, "not_found", body["error"])
}

func TestIntegration_AtualizarParaPathDuplicado_Retorna409(t *testing.T) {
	server, _ := setupIntegrationServer(t)

	r1 := postNotes(t, server.URL, `{"path":"a.md","content":"1"}`)
	require.Equal(t, http.StatusCreated, r1.StatusCode)
	_ = r1.Body.Close()

	r2 := postNotes(t, server.URL, `{"path":"b.md","content":"2"}`)
	require.Equal(t, http.StatusCreated, r2.StatusCode)
	bBody := decodeJSON(t, r2.Body)
	_ = r2.Body.Close()
	idB, _ := bBody["id"].(string)
	updatedAtStr, _ := bBody["updated_at"].(string)

	conflict := putNote(t, server.URL, idB, `{"path":"a.md","content":"x","lastModifiedAt":"`+updatedAtStr+`","force":false}`)
	defer func() { _ = conflict.Body.Close() }()
	require.Equal(t, http.StatusConflict, conflict.StatusCode)
	body := decodeJSON(t, conflict.Body)
	assert.Equal(t, "duplicate_path", body["error"])
}

func TestIntegration_ConflictoPorVersao_Force_False_Retorna409(t *testing.T) {
	server, _ := setupIntegrationServer(t)

	// Criar nota
	r1 := postNotes(t, server.URL, `{"path":"test.md","content":"v1"}`)
	require.Equal(t, http.StatusCreated, r1.StatusCode)
	body1 := decodeJSON(t, r1.Body)
	_ = r1.Body.Close()
	id, _ := body1["id"].(string)
	updatedAtStr1, _ := body1["updated_at"].(string)

	// Cliente 1 atualiza (sucesso)
	r2 := putNote(t, server.URL, id, `{"path":"test.md","content":"v2","lastModifiedAt":"`+updatedAtStr1+`","force":false}`)
	defer func() { _ = r2.Body.Close() }()
	require.Equal(t, http.StatusOK, r2.StatusCode)

	// Cliente 2 tenta atualizar com versão antiga (force=false) -> 409
	r3 := putNote(t, server.URL, id, `{"path":"test.md","content":"v3","lastModifiedAt":"`+updatedAtStr1+`","force":false}`)
	defer func() { _ = r3.Body.Close() }()
	require.Equal(t, http.StatusConflict, r3.StatusCode)
	body3 := decodeJSON(t, r3.Body)
	assert.Equal(t, "conflict", body3["error"])
}

func TestIntegration_ConflictoPorVersao_Force_True_Sucesso(t *testing.T) {
	server, db := setupIntegrationServer(t)

	// Criar nota
	r1 := postNotes(t, server.URL, `{"path":"test.md","content":"v1"}`)
	require.Equal(t, http.StatusCreated, r1.StatusCode)
	body1 := decodeJSON(t, r1.Body)
	_ = r1.Body.Close()
	id, _ := body1["id"].(string)
	updatedAtStr1, _ := body1["updated_at"].(string)

	// Cliente 1 atualiza (sucesso)
	r2 := putNote(t, server.URL, id, `{"path":"test.md","content":"v2","lastModifiedAt":"`+updatedAtStr1+`","force":false}`)
	defer func() { _ = r2.Body.Close() }()
	require.Equal(t, http.StatusOK, r2.StatusCode)

	// Cliente 2 tenta atualizar com versão antiga mas com force=true -> 200
	r3 := putNote(t, server.URL, id, `{"path":"test.md","content":"v3","lastModifiedAt":"`+updatedAtStr1+`","force":true}`)
	defer func() { _ = r3.Body.Close() }()
	require.Equal(t, http.StatusOK, r3.StatusCode)
	body3 := decodeJSON(t, r3.Body)
	assert.Equal(t, id, body3["id"])
	assert.Equal(t, "v3", body3["content"])

	// Verificar no banco
	var dbContent string
	require.NoError(t, db.QueryRow("SELECT content FROM notes WHERE id = ?", id).Scan(&dbContent))
	assert.Equal(t, "v3", dbContent)
}

func TestIntegration_DeletarNota_FluxoCompleto(t *testing.T) {
	server, db := setupIntegrationServer(t)

	created := postNotes(t, server.URL, `{"path":"a-deletar.md","content":"x"}`)
	require.Equal(t, http.StatusCreated, created.StatusCode)
	cBody := decodeJSON(t, created.Body)
	_ = created.Body.Close()
	id, _ := cBody["id"].(string)

	resp := deleteNote(t, server.URL, id)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM notes WHERE id = ?", id).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestIntegration_DeletarNotaInexistente_Retorna404(t *testing.T) {
	server, _ := setupIntegrationServer(t)

	resp := deleteNote(t, server.URL, "fantasma")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeJSON(t, resp.Body)
	assert.Equal(t, "not_found", body["error"])
}

func TestIntegration_CriarNotaDuplicada_RetornaMensagemLimpa(t *testing.T) {
	server, _ := setupIntegrationServer(t)

	body := `{"path":"dup.md","content":"x"}`
	resp1 := postNotes(t, server.URL, body)
	require.Equal(t, http.StatusCreated, resp1.StatusCode)
	_ = resp1.Body.Close()

	resp2 := postNotes(t, server.URL, body)
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, http.StatusConflict, resp2.StatusCode)

	errBody := decodeJSON(t, resp2.Body)
	assert.Equal(t, "duplicate_path", errBody["error"])
	assert.Equal(t, "path já existe", errBody["message"])

	msg, _ := errBody["message"].(string)
	assert.NotContains(t, msg, "salvar nota")
}

func TestIntegration_ListNotes_DBVazio_RetornaArrayVazio(t *testing.T) {
	server, _ := setupIntegrationServer(t)

	resp, err := http.Get(server.URL + "/notes")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var arr []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&arr))
	assert.Empty(t, arr)
}

func TestIntegration_ListNotes_RetornaArrayOrdenadoPorPath(t *testing.T) {
	server, _ := setupIntegrationServer(t)

	r1 := postNotes(t, server.URL, `{"path":"zebra.md","content":"z"}`)
	require.Equal(t, http.StatusCreated, r1.StatusCode)
	_ = r1.Body.Close()

	r2 := postNotes(t, server.URL, `{"path":"alfa.md","content":"a"}`)
	require.Equal(t, http.StatusCreated, r2.StatusCode)
	_ = r2.Body.Close()

	r3 := postNotes(t, server.URL, `{"path":"meio.md","content":"m"}`)
	require.Equal(t, http.StatusCreated, r3.StatusCode)
	_ = r3.Body.Close()

	resp, err := http.Get(server.URL + "/notes")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var arr []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&arr))
	require.Len(t, arr, 3)
	assert.Equal(t, "alfa.md", arr[0]["path"])
	assert.Equal(t, "meio.md", arr[1]["path"])
	assert.Equal(t, "zebra.md", arr[2]["path"])
	assert.Equal(t, "a", arr[0]["content"])
	assert.Equal(t, "m", arr[1]["content"])
	assert.Equal(t, "z", arr[2]["content"])
}
