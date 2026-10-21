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
	repo, _ := repoOAuth(t)

	client, err := repo.GetClient(context.Background(), oauth.BuiltinMCPClientID, time.Now())

	require.NoError(t, err)
	assert.Equal(t, oauth.ClientSourceBuiltin, client.Source)
	assert.Equal(t, []string{"http://127.0.0.1/callback", "http://[::1]/callback"}, client.RedirectURIs)
	assert.NotEmpty(t, client.Name)
}
