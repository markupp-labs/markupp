package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
)

// fakeInstanceSecrets guarda os segredos em memória no lugar do banco.
type fakeInstanceSecrets map[string]string

func (f fakeInstanceSecrets) Secret(_ context.Context, name string, generate func() (string, error)) (string, error) {
	if value, ok := f[name]; ok {
		return value, nil
	}
	value, err := generate()
	f[name] = value
	return value, err
}

func configSoComOBanco(t *testing.T, extra map[string]string) config.Config {
	t.Helper()
	env := map[string]string{"MARKUPP_DATABASE_URL": "postgres://markupp@db/markupp"}
	for key, value := range extra {
		env[key] = value
	}
	cfg, err := config.Load(func(key string) (string, bool) { v, ok := env[key]; return v, ok })
	require.NoError(t, err)
	return cfg
}

func TestPrepareInstance_EscreveOSegredoDoMCPDaMesmaOrigem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	secrets := fakeInstanceSecrets{"mcp_resource_secret": "segredo-do-mcp-com-mais-de-32-bytes"}
	cfg := configSoComOBanco(t, map[string]string{"MARKUPP_MCP_SECRET_PATH": path})

	prepared, err := prepareInstance(context.Background(), cfg, secrets)

	require.NoError(t, err)
	assert.NotEmpty(t, prepared.AuthSecret)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "segredo-do-mcp-com-mais-de-32-bytes", string(content))
}

func TestPrepareInstance_SemCaminho_NaoEscreveArquivo(t *testing.T) {
	prepared, err := prepareInstance(context.Background(), configSoComOBanco(t, nil), fakeInstanceSecrets{})

	require.NoError(t, err)
	assert.Len(t, prepared.OAuthResources, 1)
}

func TestPrepareInstance_MCPEmSubdominio_NaoEscreveArquivo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	cfg := configSoComOBanco(t, map[string]string{
		"MARKUPP_MCP_SECRET_PATH": path,
		"MARKUPP_OAUTH_RESOURCES": "https://mcp.exemplo.com/mcp=segredo-do-recurso-com-32-bytes!",
	})

	_, err := prepareInstance(context.Background(), cfg, fakeInstanceSecrets{})

	require.NoError(t, err)
	assert.NoFileExists(t, path)
}
