package loginprovider_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

// fakeGitHubServer faz o papel do OAuth e da API do GitHub. Como o GitHub, recusa o
// código errado com 200 e o campo error.
type fakeGitHubServer struct {
	*httptest.Server
	wantCode string
	userID   int64
	emails   []map[string]any
}

func newFakeGitHubServer(t *testing.T) *fakeGitHubServer {
	t.Helper()
	fake := &fakeGitHubServer{wantCode: "codigo-valido", userID: 42, emails: []map[string]any{
		{"email": "outro@exemplo.org", "primary": false, "verified": true},
		{"email": "ana@exemplo.org", "primary": true, "verified": true},
	}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/oauth/access_token", fake.accessToken)
	mux.HandleFunc("GET /api/user", fake.requireToken(func(w http.ResponseWriter) {
		writeTestJSON(w, map[string]any{"id": fake.userID, "login": "ana"})
	}))
	mux.HandleFunc("GET /api/user/emails", fake.requireToken(func(w http.ResponseWriter) { writeTestJSON(w, fake.emails) }))
	fake.Server = httptest.NewServer(mux)
	t.Cleanup(fake.Close)
	return fake
}

func (f *fakeGitHubServer) endpoints() loginprovider.GitHubEndpoints {
	return loginprovider.GitHubEndpoints{
		AuthURL:  f.URL + "/login/oauth/authorize",
		TokenURL: f.URL + "/login/oauth/access_token",
		APIURL:   f.URL + "/api",
	}
}

func (f *fakeGitHubServer) accessToken(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("code") != f.wantCode || r.FormValue("code_verifier") == "" {
		writeTestJSON(w, map[string]string{"error": "bad_verification_code"})
		return
	}
	writeTestJSON(w, map[string]string{"access_token": "token-github", "token_type": "bearer"})
}

func (f *fakeGitHubServer) requireToken(next func(http.ResponseWriter)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-github" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next(w)
	}
}
