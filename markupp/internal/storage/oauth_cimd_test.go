package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

const clienteCIMD = "https://agente.example/oauth/client.json"

func documentoCIMD(nome string, redirects ...string) oauth.Client {
	return oauth.Client{ID: clienteCIMD, Name: nome, RedirectURIs: redirects}
}

func TestPostgresOAuthRepo_SaveCIMDClient_CriaCliente(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	cliente := documentoCIMD("Agente CIMD", "https://agente.example/cb")

	require.NoError(t, repo.SaveCIMDClient(ctx, cliente, instanteOAuth.Add(time.Hour)))

	got, err := repo.GetFreshCIMDClient(ctx, clienteCIMD, instanteOAuth)
	require.NoError(t, err)
	assert.Equal(t, cliente, got)
}

func TestPostgresOAuthRepo_SaveCIMDClient_AtualizaDocumento(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	require.NoError(t, repo.SaveCIMDClient(ctx, documentoCIMD("Antigo", "https://agente.example/cb"), instanteOAuth.Add(time.Minute)))
	novo := documentoCIMD("Novo", "https://agente.example/cb2", "http://127.0.0.1/cb")

	require.NoError(t, repo.SaveCIMDClient(ctx, novo, instanteOAuth.Add(2*time.Hour)))

	got, err := repo.GetFreshCIMDClient(ctx, clienteCIMD, instanteOAuth.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, novo, got)
}

func TestPostgresOAuthRepo_SaveCIMDClient_NaoSobrescreveClienteDCR(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	intruso := oauth.Client{ID: "c1", Name: "Intruso", RedirectURIs: []string{"https://intruso.example/cb"}}

	err := repo.SaveCIMDClient(ctx, intruso, instanteOAuth.Add(time.Hour))

	require.ErrorIs(t, err, oauth.ErrClientSourceConflict)
	assert.ErrorContains(t, err, `esperado source "cimd"`)
	got, err := repo.GetClient(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, oauth.Client{ID: "c1", Name: "Agente", RedirectURIs: []string{"http://127.0.0.1/cb"}}, got)
	assert.Equal(t, 1, countRows(t, pool, `SELECT count(*) FROM oauth_clients
		WHERE id = 'c1' AND source = 'dcr' AND metadata_expires_at IS NULL`))
}

func TestPostgresOAuthRepo_GetFreshCIMDClient_IgnoraVencido(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	expira := instanteOAuth.Add(time.Hour)
	require.NoError(t, repo.SaveCIMDClient(ctx, documentoCIMD("Agente CIMD", "https://agente.example/cb"), expira))

	_, err := repo.GetFreshCIMDClient(ctx, clienteCIMD, expira)

	require.ErrorIs(t, err, oauth.ErrClientNotFound)
}
