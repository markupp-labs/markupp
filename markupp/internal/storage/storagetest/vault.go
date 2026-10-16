package storagetest

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/benchfixture"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

const insertVaultNotes = `INSERT INTO notes (id, tenant_id, vault_id, path, content, created_at, updated_at)
SELECT n.id, $1, $2, n.path, n.content, now(), now()
FROM unnest($3::text[], $4::text[], $5::text[]) AS n (id, path, content)`

// SeedVault semeia o ator no cofre vaultID e grava as notas do cofre sintético num
// comando só, com o papel dono do banco. Os triggers rodam como numa escrita real,
// então todas as notas entram na fila do indexador. A nota i ganha o id
// "<vaultID>-<i>".
func SeedVault(t testing.TB, pool *pgxpool.Pool, actor identity.Actor, vaultID string, vault []benchfixture.Note) context.Context {
	t.Helper()
	ctx := SeedMember(t, pool, actor, vaultID)
	ids := make([]string, len(vault))
	paths := make([]string, len(vault))
	contents := make([]string, len(vault))
	for i, note := range vault {
		ids[i], paths[i], contents[i] = fmt.Sprintf("%s-%05d", vaultID, i), note.Path, note.Content
	}
	_, err := pool.Exec(context.Background(), insertVaultNotes, actor.TenantID, vaultID, ids, paths, contents)
	require.NoError(t, err, "semear %d notas no cofre %q", len(vault), vaultID)
	return ctx
}
