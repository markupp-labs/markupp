package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

var tabelasOAuth = []string{"oauth_clients", "oauth_authorizations", "oauth_requests", "oauth_tokens"}

// bancoComOAuth sobe todas as migrações e grava uma conta e um cliente OAuth.
func bancoComOAuth(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := storagetest.EmptyPool(t)
	_, err := gooseProvider(t, pool).Up(context.Background())
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `INSERT INTO users (id, email, created_at, updated_at)
		VALUES ('u1', 'u1@markupp.dev', now(), now())`)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `INSERT INTO oauth_clients (id, source, name, redirect_uris)
		VALUES ('c1', 'dcr', 'Agente', ARRAY['http://127.0.0.1/cb'])`)
	require.NoError(t, err)
	return pool
}

func autorizar(pool *pgxpool.Pool, id, escopos string, revogada bool) error {
	_, err := pool.Exec(context.Background(), `INSERT INTO oauth_authorizations (id, user_id, client_id, scopes, revoked_at)
		VALUES ($1, 'u1', 'c1', $2::TEXT[], CASE WHEN $3 THEN now() END)`, id, escopos, revogada)
	return err
}

func TestMigracao00016_TabelasOAuth_ExistemSemRLS(t *testing.T) {
	pool := bancoComOAuth(t)

	for _, tabela := range tabelasOAuth {
		assert.Equal(t, 1, countRows(t, pool,
			"SELECT count(*) FROM pg_class WHERE relname = '"+tabela+"' AND relkind = 'r' AND NOT relrowsecurity"),
			"tabela %s deve existir sem RLS", tabela)
	}
}

func TestMigracao00016_TabelasOAuth_SemPrivilegioParaApp(t *testing.T) {
	pool := bancoComOAuth(t)

	for _, tabela := range tabelasOAuth {
		assert.Equal(t, 0, countRows(t, pool, `SELECT count(*) FROM unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE',
			'TRUNCATE','REFERENCES','TRIGGER']) p WHERE has_table_privilege('markupp_app', '`+tabela+`', p)`),
			"markupp_app não pode tocar em %s", tabela)
	}
}

func TestMigracao00016_EscopoInvalido_Rejeitado(t *testing.T) {
	pool := bancoComOAuth(t)

	assert.Error(t, autorizar(pool, "a1", "{notas:tudo}", false))
	assert.Error(t, autorizar(pool, "a2", "{}", false), "lista vazia de escopos")
	assert.NoError(t, autorizar(pool, "a3", "{notas:leitura,notas:escrita,administracao}", false))
}

func TestMigracao00016_SourceInvalido_Rejeitado(t *testing.T) {
	pool := bancoComOAuth(t)

	_, err := pool.Exec(context.Background(), `INSERT INTO oauth_clients (id, source, name, redirect_uris)
		VALUES ('c2', 'manual', 'Agente', ARRAY['http://127.0.0.1/cb'])`)

	assert.Error(t, err)
}

func TestMigracao00016_DuasAutorizacoesAtivasDoMesmoPar_Rejeitadas(t *testing.T) {
	pool := bancoComOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", false))

	assert.Error(t, autorizar(pool, "a2", "{notas:leitura}", false))
}

func TestMigracao00016_AtivaMaisRevogadaDoMesmoPar_Aceitas(t *testing.T) {
	pool := bancoComOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", true))

	assert.NoError(t, autorizar(pool, "a2", "{notas:leitura}", false))
}

func TestMigracao00016_Down_RemoveTabelasOAuth(t *testing.T) {
	pool := bancoComOAuth(t)
	provider := gooseProvider(t, pool)

	_, err := provider.DownTo(context.Background(), 15)

	require.NoError(t, err)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM pg_class WHERE relname LIKE 'oauth\\_%'"))
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
}
