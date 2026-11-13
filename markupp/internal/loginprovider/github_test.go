package loginprovider_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

func novoGitHub(fake *fakeGitHubServer) loginprovider.Provider {
	return loginprovider.NewGitHub(loginprovider.Settings{
		ID: "github", Name: "GitHub", ClientID: "id-github", ClientSecret: "segredo",
		RedirectURL: "https://painel.exemplo.org/auth/github/callback",
	}, fake.endpoints(), fake.Client())
}

func TestGitHub_Begin_MontaAURLComStateEPKCE(t *testing.T) {
	fake := newFakeGitHubServer(t)

	attempt, err := novoGitHub(fake).Begin(context.Background())

	require.NoError(t, err)
	parsed, err := url.Parse(attempt.AuthorizationURL)
	require.NoError(t, err)
	q := parsed.Query()
	assert.Equal(t, fake.URL+"/login/oauth/authorize", parsed.Scheme+"://"+parsed.Host+parsed.Path)
	assert.Equal(t, "id-github", q.Get("client_id"))
	assert.Equal(t, "https://painel.exemplo.org/auth/github/callback", q.Get("redirect_uri"))
	assert.Equal(t, "read:user user:email", q.Get("scope"))
	assert.Equal(t, attempt.State, q.Get("state"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.NotEmpty(t, attempt.CodeVerifier)
}

func TestGitHub_Finish_DevolveOIdENumericoEOEmailPrincipal(t *testing.T) {
	fake := newFakeGitHubServer(t)

	got, err := novoGitHub(fake).Finish(context.Background(), "codigo-valido", "verifier", "")

	require.NoError(t, err)
	assert.Equal(t, loginprovider.Identity{Subject: "42", Email: "ana@exemplo.org", EmailVerified: true}, got)
}

func TestGitHub_Finish_EmailPrincipalNaoVerificado_DevolveSemVerificacao(t *testing.T) {
	fake := newFakeGitHubServer(t)
	fake.emails = []map[string]any{{"email": "ana@exemplo.org", "primary": true, "verified": false}}

	got, err := novoGitHub(fake).Finish(context.Background(), "codigo-valido", "verifier", "")

	require.NoError(t, err)
	assert.False(t, got.EmailVerified)
}

func TestGitHub_Finish_CodigoErrado_Recusa(t *testing.T) {
	fake := newFakeGitHubServer(t)

	_, err := novoGitHub(fake).Finish(context.Background(), "outro", "verifier", "")

	require.ErrorIs(t, err, loginprovider.ErrLoginRejected)
}

func TestGitHub_Finish_ForaDoAr_DevolveIndisponivel(t *testing.T) {
	fake := newFakeGitHubServer(t)
	provider := novoGitHub(fake)
	fake.Close()

	_, err := provider.Finish(context.Background(), "codigo-valido", "verifier", "")

	require.ErrorIs(t, err, loginprovider.ErrProviderUnavailable)
}
