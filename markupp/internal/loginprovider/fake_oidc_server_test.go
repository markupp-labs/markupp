package loginprovider_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// fakeOIDCServer faz o papel de um provedor OpenID Connect: descoberta, JWKS, token e
// um id_token assinado com as claims que o teste escolhe.
type fakeOIDCServer struct {
	*httptest.Server
	key          *rsa.PrivateKey
	wantCode     string
	wantVerifier string
	claims       map[string]any
	discoveries  atomic.Int32
}

func newFakeOIDCServer(t *testing.T) *fakeOIDCServer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	fake := &fakeOIDCServer{key: key, wantCode: "codigo-valido"}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", fake.discovery)
	mux.HandleFunc("GET /jwks", fake.jwks)
	mux.HandleFunc("POST /token", fake.token)
	fake.Server = httptest.NewServer(mux)
	t.Cleanup(fake.Close)
	fake.claims = map[string]any{"iss": fake.URL, "aud": "id-do-cliente", "sub": "sujeito-1",
		"email": "ana@exemplo.org", "email_verified": true}
	return fake
}

func (f *fakeOIDCServer) discovery(w http.ResponseWriter, _ *http.Request) {
	f.discoveries.Add(1)
	writeTestJSON(w, map[string]any{
		"issuer": f.URL, "authorization_endpoint": f.URL + "/authorize", "token_endpoint": f.URL + "/token",
		"jwks_uri": f.URL + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (f *fakeOIDCServer) jwks(w http.ResponseWriter, _ *http.Request) {
	pub := f.key.PublicKey
	writeTestJSON(w, map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "chave-1",
		"n": b64(pub.N.Bytes()), "e": b64(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

// token troca o código pelo id_token só com o código e o verifier PKCE esperados.
func (f *fakeOIDCServer) token(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("code") != f.wantCode || (f.wantVerifier != "" && r.FormValue("code_verifier") != f.wantVerifier) {
		w.WriteHeader(http.StatusBadRequest)
		writeTestJSON(w, map[string]string{"error": "invalid_grant"})
		return
	}
	writeTestJSON(w, map[string]any{"access_token": "acesso", "token_type": "Bearer", "expires_in": 300, "id_token": f.signIDToken()})
}

func (f *fakeOIDCServer) signIDToken() string {
	claims := map[string]any{"iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix()}
	for k, v := range f.claims {
		claims[k] = v
	}
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "chave-1", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	signingInput := b64(header) + "." + b64(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, _ := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	return signingInput + "." + b64(signature)
}

func b64(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func writeTestJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}
