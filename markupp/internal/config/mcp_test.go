package config_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
)

// FakeInstanceSecrets guarda os segredos em memória, como o banco guardaria, e conta
// quantas vezes cada nome foi gerado.
type FakeInstanceSecrets struct {
	values    map[string]string
	generated []string
}

func newFakeInstanceSecrets() *FakeInstanceSecrets {
	return &FakeInstanceSecrets{values: map[string]string{}}
}

func (f *FakeInstanceSecrets) Secret(_ context.Context, name string, generate func() (string, error)) (string, error) {
	if value, ok := f.values[name]; ok {
		return value, nil
	}
	value, err := generate()
	if err != nil {
		return "", err
	}
	f.generated = append(f.generated, name)
	f.values[name] = value
	return value, nil
}

func gerarSegredo() (string, error) { return "segredo-gerado-com-mais-de-32-bytes!", nil }

func carregar(t *testing.T, env fakeEnv) config.Config {
	t.Helper()
	cfg, err := config.Load(env.Lookup)
	require.NoError(t, err)
	return cfg
}

func TestLoad_LeOUpstreamEOArquivoDoSegredoDoMCP(t *testing.T) {
	cfg := carregar(t, fakeEnv{ // #nosec G101 -- caminho fictício de teste, nao o segredo
		"MARKUPP_DATABASE_URL":     urlDeTeste,
		"MARKUPP_MCP_UPSTREAM_URL": "http://mcp:3000/",
		"MARKUPP_MCP_SECRET_PATH":  "/run/markupp-mcp/secret",
	})

	assert.Equal(t, "http://mcp:3000", cfg.MCPUpstreamURL)
	assert.Equal(t, "/run/markupp-mcp/secret", cfg.MCPSecretPath)
}

func TestLoad_UpstreamDoMCPForaDoFormato_RetornaErroComOValor(t *testing.T) {
	for _, valor := range []string{"ftp://mcp", "mcp:3000", "http://", "http://mcp:3000/x?a=1"} {
		env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_MCP_UPSTREAM_URL": valor}

		_, err := config.Load(env.Lookup)

		require.Error(t, err, valor)
		assert.Contains(t, err.Error(), "MARKUPP_MCP_UPSTREAM_URL", valor)
		assert.Contains(t, err.Error(), "http(s)://host[:porta]", valor)
	}
}

func TestWithInstanceSecrets_SemVariaveis_GeraOSegredoDoJWTEODoMCP(t *testing.T) {
	secrets := newFakeInstanceSecrets()
	cfg := carregar(t, fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste})

	resolved, err := cfg.WithInstanceSecrets(context.Background(), secrets, gerarSegredo)

	require.NoError(t, err)
	assert.Equal(t, "segredo-gerado-com-mais-de-32-bytes!", resolved.AuthSecret)
	assert.Equal(t, []config.OAuthResource{{URL: "http://localhost:8080/mcp", Secret: "segredo-gerado-com-mais-de-32-bytes!"}}, resolved.OAuthResources) // #nosec G101 -- valor fictício de teste
	assert.ElementsMatch(t, []string{"auth_secret", "mcp_resource_secret"}, secrets.generated)
}

func TestWithInstanceSecrets_ComVariaveis_UsaOOverrideSemGerar(t *testing.T) {
	secrets := newFakeInstanceSecrets()
	env := envComOAuth()

	resolved, err := carregar(t, env).WithInstanceSecrets(context.Background(), secrets, gerarSegredo)

	require.NoError(t, err)
	assert.Equal(t, segredoDeTesteValido, resolved.AuthSecret)
	assert.Equal(t, "https://mcp.markupp.dev/mcp", resolved.OAuthResources[0].URL)
	assert.Empty(t, secrets.generated)
}

func TestWithInstanceSecrets_FalhaDoBanco_RetornaErroComONome(t *testing.T) {
	falha := func() (string, error) { return "", errors.New("banco fora") }

	_, err := carregar(t, fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste}).
		WithInstanceSecrets(context.Background(), newFakeInstanceSecrets(), falha)

	require.ErrorContains(t, err, "banco fora")
	require.ErrorContains(t, err, "auth_secret")
}

func TestDefaultMCPResourceSecret_AchaOSegredoDoMCPNaMesmaOrigem(t *testing.T) {
	resolved, err := carregar(t, fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste}).
		WithInstanceSecrets(context.Background(), newFakeInstanceSecrets(), gerarSegredo)
	require.NoError(t, err)

	secret, ok := resolved.DefaultMCPResourceSecret()

	assert.True(t, ok)
	assert.Equal(t, "segredo-gerado-com-mais-de-32-bytes!", secret)
}

func TestDefaultMCPResourceSecret_SemOMCPNaMesmaOrigem_NaoAcha(t *testing.T) {
	_, ok := carregar(t, envComOAuth()).DefaultMCPResourceSecret()

	assert.False(t, ok, "o MCP em subdominio recebe o segredo pelo Secret do cluster")
}

func TestWriteMCPSecret_GravaSoOSegredoESubstituiOAnterior(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp", "secret")

	require.NoError(t, config.WriteMCPSecret(path, "primeiro"))
	require.NoError(t, config.WriteMCPSecret(path, "segundo"))

	content, err := os.ReadFile(path) // #nosec G304 -- arquivo do t.TempDir do proprio teste
	require.NoError(t, err)
	assert.Equal(t, "segundo", string(content))
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "a escrita atomica nao deixa arquivo temporario")
}

func TestWriteMCPSecret_DiretorioInacessivel_RetornaErroComOCaminho(t *testing.T) {
	bloqueado := filepath.Join(t.TempDir(), "arquivo")
	require.NoError(t, os.WriteFile(bloqueado, nil, 0o600))

	err := config.WriteMCPSecret(filepath.Join(bloqueado, "secret"), "x")

	require.ErrorContains(t, err, bloqueado)
}
