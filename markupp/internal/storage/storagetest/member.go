package storagetest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

// SeedMember grava, com o papel dono do banco, o usuário do ator, o cofre vaultID no
// tenant dele e o vínculo entre os dois. Devolve um contexto que carrega o ator.
func SeedMember(t *testing.T, pool *pgxpool.Pool, actor identity.Actor, vaultID string) context.Context {
	t.Helper()
	ctx := context.Background()
	seed := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users (id, tenant_id, email, role, created_at, updated_at)
		  VALUES ($1, $2, $1 || '@markupp.dev', $3, now(), now()) ON CONFLICT DO NOTHING`,
			[]any{actor.UserID, actor.TenantID, actor.Role}},
		{`INSERT INTO vaults (id, tenant_id, name, created_at)
		  VALUES ($1, $2, $1, clock_timestamp()) ON CONFLICT DO NOTHING`,
			[]any{vaultID, actor.TenantID}},
		{`INSERT INTO vault_members (vault_id, user_id, created_at)
		  VALUES ($1, $2, clock_timestamp()) ON CONFLICT DO NOTHING`,
			[]any{vaultID, actor.UserID}},
	}
	for _, s := range seed {
		_, err := pool.Exec(ctx, s.sql, s.args...)
		require.NoError(t, err, "semear membro %+v no cofre %q", actor, vaultID)
	}
	return identity.WithActor(ctx, actor)
}
