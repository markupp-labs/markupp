package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func TestPostgresOAuthRepo_ClienteEmbutido_ExisteDeFabricaComRedirectEmLoopback(t *testing.T) {
	assertClienteEmbutidoDeFabrica(t, oauth.BuiltinMCPClientID)
}

func TestPostgresOAuthRepo_ClienteEmbutidoDoCLI_ExisteDeFabricaComRedirectEmLoopback(t *testing.T) {
	assertClienteEmbutidoDeFabrica(t, oauth.BuiltinCLIClientID)
}

func assertClienteEmbutidoDeFabrica(t *testing.T, clientID string) {
	t.Helper()
	repo, _ := repoOAuth(t)

	client, err := repo.GetClient(context.Background(), clientID, time.Now())

	require.NoError(t, err)
	assert.Equal(t, oauth.ClientSourceBuiltin, client.Source)
	assert.Equal(t, []string{"http://127.0.0.1/callback", "http://[::1]/callback"}, client.RedirectURIs)
	assert.NotEmpty(t, client.Name)
}
