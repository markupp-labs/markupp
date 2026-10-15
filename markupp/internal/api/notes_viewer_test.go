package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

func notesRequestAs(role, method, path, body string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Notes:   &fakeService{note: notes.Note{ID: "id-x"}},
		Tokens:  &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}},
		Tenants: &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: role}},
	})
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestNotes_Viewer_EscritaRetorna403ELeituraSegue(t *testing.T) {
	escritas := [][3]string{
		{http.MethodPost, "/notes", `{"path":"x.md","content":"y"}`},
		{http.MethodPut, "/notes/id-x", `{"path":"x.md","content":"y","force":true}`},
		{http.MethodDelete, "/notes/id-x", ""},
	}
	for _, e := range escritas {
		rec := notesRequestAs("viewer", e[0], e[1], e[2])

		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s", e[0], e[1])
		assert.Contains(t, rec.Body.String(), "read_only")
	}
	assert.Equal(t, http.StatusOK, notesRequestAs("viewer", http.MethodGet, "/notes", "").Code)
	assert.Equal(t, http.StatusCreated, notesRequestAs("member", http.MethodPost, "/notes", `{"path":"x.md","content":"y"}`).Code)
}
