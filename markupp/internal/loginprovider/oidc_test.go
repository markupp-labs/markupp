package loginprovider_test

import (
	"context"
	"crypto/sha256"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

func novoOIDC(fake *fakeOIDCServer) loginprovider.Provider {
	return loginprovider.NewOIDC(loginprovider.Settings{
		ID: "idp", Name: "Meu IdP", Issuer: fake.URL, ClientID: "id-do-cliente", ClientSecret: "segredo",
		RedirectURL: "https://painel.exemplo.org/auth/idp/callback",
	}, fake.Client())
}

func TestOIDC_Construir_NaoBuscaADescoberta(t *testing.T) {
	fake := newFakeOIDCServer(t)

	novoOIDC(fake)

	assert.Zero(t, fake.discoveries.Load(), "IdP fora do ar nao pode impedir o servidor de subir")
}

func TestOIDC_Begin_MontaAURLComStateNonceEPKCE(t *testing.T) {
	fake := newFakeOIDCServer(t)

	attempt, err := novoOIDC(fake).Begin(context.Background())

	require.NoError(t, err)
	parsed, err := url.Parse(attempt.AuthorizationURL)
	require.NoError(t, err)
	q := parsed.Query()
	assert.Equal(t, fake.URL+"/authorize", parsed.Scheme+"://"+parsed.Host+parsed.Path)
	assert.Equal(t, "id-do-cliente", q.Get("client_id"))
	assert.Equal(t, "https://painel.exemplo.org/auth/idp/callback", q.Get("redirect_uri"))
	assert.Equal(t, "openid email profile", q.Get("scope"))
	assert.Equal(t, attempt.State, q.Get("state"))
	assert.Equal(t, attempt.Nonce, q.Get("nonce"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	digest := sha256.Sum256([]byte(attempt.CodeVerifier))
	assert.Equal(t, b64(digest[:]), q.Get("code_challenge"))
	assert.NotEmpty(t, attempt.State)
	assert.NotEmpty(t, attempt.Nonce)
}

func TestOIDC_Begin_DuasVezes_GeraValoresDiferentes(t *testing.T) {
	provider := novoOIDC(newFakeOIDCServer(t))

	first, err := provider.Begin(context.Background())
	require.NoError(t, err)
	second, err := provider.Begin(context.Background())
	require.NoError(t, err)

	assert.NotEqual(t, first.State, second.State)
	assert.NotEqual(t, first.Nonce, second.Nonce)
	assert.NotEqual(t, first.CodeVerifier, second.CodeVerifier)
}

func TestOIDC_Begin_DescobreUmaVezSo(t *testing.T) {
	fake := newFakeOIDCServer(t)
	provider := novoOIDC(fake)

	_, _ = provider.Begin(context.Background())
	_, _ = provider.Begin(context.Background())

	assert.Equal(t, int32(1), fake.discoveries.Load())
}

func TestOIDC_Begin_IdPForaDoAr_DevolveIndisponivel(t *testing.T) {
	fake := newFakeOIDCServer(t)
	provider := novoOIDC(fake)
	fake.Close()

	_, err := provider.Begin(context.Background())

	require.ErrorIs(t, err, loginprovider.ErrProviderUnavailable)
}

// comecar faz o Begin e prepara o fake para aceitar o verifier e o nonce dessa tentativa.
func comecar(t *testing.T, fake *fakeOIDCServer, provider loginprovider.Provider) loginprovider.Attempt {
	t.Helper()
	attempt, err := provider.Begin(context.Background())
	require.NoError(t, err)
	fake.wantVerifier = attempt.CodeVerifier
	fake.claims["nonce"] = attempt.Nonce
	return attempt
}

func TestOIDC_Finish_DevolveAIdentidadeDoIDToken(t *testing.T) {
	fake := newFakeOIDCServer(t)
	provider := novoOIDC(fake)
	attempt := comecar(t, fake, provider)

	got, err := provider.Finish(context.Background(), "codigo-valido", attempt.CodeVerifier, attempt.Nonce)

	require.NoError(t, err)
	assert.Equal(t, loginprovider.Identity{Subject: "sujeito-1", Email: "ana@exemplo.org", EmailVerified: true}, got)
}

func TestOIDC_Finish_EmailNaoVerificado_DevolveSemVerificacao(t *testing.T) {
	fake := newFakeOIDCServer(t)
	provider := novoOIDC(fake)
	attempt := comecar(t, fake, provider)
	fake.claims["email_verified"] = false

	got, err := provider.Finish(context.Background(), "codigo-valido", attempt.CodeVerifier, attempt.Nonce)

	require.NoError(t, err)
	assert.False(t, got.EmailVerified)
}

func TestOIDC_Finish_Recusas(t *testing.T) {
	casos := map[string]func(fake *fakeOIDCServer, code, verifier, nonce *string){
		"codigo errado":      func(_ *fakeOIDCServer, code, _, _ *string) { *code = "outro" },
		"verifier errado":    func(_ *fakeOIDCServer, _, verifier, _ *string) { *verifier = "outro-verifier" },
		"nonce de outra vez": func(_ *fakeOIDCServer, _, _, nonce *string) { *nonce = "outro-nonce" },
		"audiencia de outro": func(fake *fakeOIDCServer, _, _, _ *string) { fake.claims["aud"] = "outro-cliente" },
		"emissor de outro":   func(fake *fakeOIDCServer, _, _, _ *string) { fake.claims["iss"] = "https://outro.exemplo.org" },
		"sem nonce no token": func(fake *fakeOIDCServer, _, _, _ *string) { delete(fake.claims, "nonce") },
	}
	for nome, estragar := range casos {
		t.Run(nome, func(t *testing.T) {
			fake := newFakeOIDCServer(t)
			provider := novoOIDC(fake)
			attempt := comecar(t, fake, provider)
			code, verifier, nonce := "codigo-valido", attempt.CodeVerifier, attempt.Nonce
			estragar(fake, &code, &verifier, &nonce)

			_, err := provider.Finish(context.Background(), code, verifier, nonce)

			require.ErrorIs(t, err, loginprovider.ErrLoginRejected)
		})
	}
}

func TestOIDC_Finish_NonceVazio_Recusa(t *testing.T) {
	fake := newFakeOIDCServer(t)
	provider := novoOIDC(fake)
	attempt := comecar(t, fake, provider)
	fake.claims["nonce"] = ""

	_, err := provider.Finish(context.Background(), "codigo-valido", attempt.CodeVerifier, "")

	require.ErrorIs(t, err, loginprovider.ErrLoginRejected, "nonce vazio nao amarra o token a tentativa")
}

func TestOIDC_NomeEId(t *testing.T) {
	provider := novoOIDC(newFakeOIDCServer(t))

	assert.Equal(t, "idp", provider.ID())
	assert.Equal(t, "Meu IdP", provider.Name())
}
