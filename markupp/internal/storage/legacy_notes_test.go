package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
	"github.com/ifsc-ES2/projeto-markupp/markupp/migrations"
)

// bancoDaRC1ComNota migra só até a versão 2, que é o schema da v1.0.0-rc.1 com a
// autenticação, e grava uma nota ainda sem cofre.
func bancoDaRC1ComNota(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool := storagetest.EmptyPool(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	require.NoError(t, err)
	_, err = provider.UpTo(ctx, 2)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO notes (id, path, content, created_at, updated_at) VALUES ('antiga', 'antiga.md', 'da rc1', now(), now())")
	require.NoError(t, err)
	return pool
}

func TestMigracao_NotaDaRC1_PrimeiroAdminEnxergaNoCofreImportadas(t *testing.T) {
	db := bancoDaRC1ComNota(t)
	require.NoError(t, storage.Migrate(context.Background(), db))
	params := sampleAccountParams()
	params.User.TenantID = "default"
	_, err := storage.NewPostgresAuthRepository(db).CreateAccountTx(context.Background(), params)
	require.NoError(t, err)

	ctx := identity.WithActor(context.Background(), identity.Actor{UserID: params.User.ID, TenantID: "default", Role: "admin"})
	lista, err := storage.NewPostgresNotesRepository(db).ListNotes(ctx)

	require.NoError(t, err)
	require.Len(t, lista, 1)
	assert.Equal(t, "antiga.md", lista[0].Path)
	assert.Equal(t, "importadas", lista[0].VaultID)
}

func TestCreateAccountTx_SegundoUsuario_NaoEntraNosCofresExistentes(t *testing.T) {
	db := bancoDaRC1ComNota(t)
	require.NoError(t, storage.Migrate(context.Background(), db))
	repo := storage.NewPostgresAuthRepository(db)
	admin := sampleAccountParams()
	admin.User.TenantID = "default"
	_, err := repo.CreateAccountTx(context.Background(), admin)
	require.NoError(t, err)
	membro := segundaContaNoTenant(admin)
	_, err = repo.CreateAccountTx(context.Background(), membro)
	require.NoError(t, err)

	ctx := identity.WithActor(context.Background(), identity.Actor{UserID: membro.User.ID, TenantID: "default", Role: "member"})
	lista, err := storage.NewPostgresNotesRepository(db).ListNotes(ctx)

	require.NoError(t, err)
	assert.Empty(t, lista, "so o primeiro admin herda o cofre importado")
}

func segundaContaNoTenant(primeira auth.CreateAccountParams) auth.CreateAccountParams {
	p := primeira
	p.User.ID, p.User.Email = "user-id-2", "membro@markupp.dev"
	p.Identity.ID, p.Identity.UserID, p.Identity.ProviderUserID = "identity-id-2", "user-id-2", "membro@markupp.dev"
	p.InitialVaultID, p.AuditEventID = "vault-id-2", "audit-id-2"
	return p
}
