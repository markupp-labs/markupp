package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
	"github.com/ifsc-ES2/projeto-markupp/markupp/migrations"
)

func gooseProvider(t *testing.T, pool *pgxpool.Pool) *goose.Provider {
	t.Helper()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	require.NoError(t, err)
	return provider
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), query).Scan(&n))
	return n
}

func TestMigracao00005_TenantSemAdmin_GanhaOwner(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.UpTo(context.Background(), 4)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `INSERT INTO users (id, tenant_id, email, role, created_at, updated_at)
		VALUES ('antigo', 'sem-admin', 'a@x.dev', 'member', now() - interval '1 day', now()),
		       ('novo', 'sem-admin', 'b@x.dev', 'member', now(), now())`)
	require.NoError(t, err)

	_, err = provider.Up(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM tenant_memberships WHERE user_id = 'antigo' AND role = 'owner'"))
}

func TestMigracoes_DownEUpComContaEmDoisTenants(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.Up(context.Background())
	require.NoError(t, err)
	dona := identity.Actor{UserID: "dona", TenantID: "t1", Role: "owner"}
	storagetest.SeedMember(t, pool, dona, "va")
	ctxT2 := storagetest.SeedMember(t, pool, identity.Actor{UserID: "dona", TenantID: "t2", Role: "owner"}, "vb")
	emitirConvite(ctxT2, t, pool, conviteDe(identity.Actor{UserID: "dona", TenantID: "t2", Role: "owner"}, "c-t2", "x@x.dev"), "tok")

	_, err = provider.DownTo(context.Background(), 4)
	require.NoError(t, err, "o down volta a conta ao tenant mais antigo sem quebrar convite do outro")
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
}

func TestMigracao00013_NotasAnteriores_FicamPendentesDeIndexacao(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.UpTo(context.Background(), 12)
	require.NoError(t, err)
	storagetest.SeedMember(t, pool, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	_, err = pool.Exec(context.Background(), `INSERT INTO notes (id, tenant_id, vault_id, path, content, created_at, updated_at)
		VALUES ('antiga', 't1', 'va', 'antiga.md', 'oi', now(), now())`)
	require.NoError(t, err)

	_, err = provider.Up(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, pool, "SELECT markupp_pending_vaults()"),
		"cofre com nota de antes da migração precisa ser indexado")
}

func TestMigracao00013_NaoReescreveATabelaDeNotas(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.UpTo(context.Background(), 12)
	require.NoError(t, err)
	const arquivoDasNotas = "SELECT pg_relation_filenode('notes')::bigint"
	antes := countRows(t, pool, arquivoDasNotas)

	_, err = provider.Up(context.Background())

	require.NoError(t, err)
	assert.Equal(t, antes, countRows(t, pool, arquivoDasNotas),
		"reescrever notas trava leitura e escrita durante o upgrade")
}

func TestMigracao00013_DownEUp(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.Up(context.Background())
	require.NoError(t, err)
	storagetest.SeedMember(t, pool, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")

	_, err = provider.DownTo(context.Background(), 12)
	require.NoError(t, err)
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
}

func TestMigracao00014_DownEUpComNotificacaoNaFila(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.Up(context.Background())
	require.NoError(t, err)
	conviteComEmail(t, pool, "c1", "nova@markupp.dev")

	_, err = provider.DownTo(context.Background(), 13)
	require.NoError(t, err)
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
}
