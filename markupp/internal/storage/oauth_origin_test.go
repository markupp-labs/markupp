package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func TestPostgresOAuthRepo_TouchAuthorization_ListaMostraOrigemEUltimoUso(t *testing.T) {
	repo, pool := repoOAuth(t)
	autorizacoesDeVariasContas(t, pool)
	quando := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	origem := auth.Origin{IP: "198.51.100.9", UserAgent: "Claude-User/1.0"}

	require.NoError(t, repo.TouchAuthorization(context.Background(), "a1", origem, quando))

	got, err := repo.ListActiveAuthorizations(context.Background(), "u1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a1", got[1].ID, "o uso não mexe na ordem, que é pela mudança da Autorização")
	assert.Equal(t, origem, got[1].Origin)
	assert.True(t, quando.Equal(got[1].LastUsedAt))
	assert.True(t, got[0].LastUsedAt.IsZero(), "Autorização nunca usada")
}

func TestPostgresOAuthRepo_AccountBlocked(t *testing.T) {
	repo, pool := repoOAuth(t)
	autorizacoesDeVariasContas(t, pool)
	_, err := pool.Exec(context.Background(), `UPDATE users SET blocked_at = now() WHERE id = 'u2'`)
	require.NoError(t, err)

	livre, err := repo.AccountBlocked(context.Background(), "u1")
	require.NoError(t, err)
	bloqueada, err := repo.AccountBlocked(context.Background(), "u2")
	require.NoError(t, err)

	assert.False(t, livre)
	assert.True(t, bloqueada)
}
