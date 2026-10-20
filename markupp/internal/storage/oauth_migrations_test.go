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

func TestMigracao00032_TabelasOAuth_ExistemSemRLS(t *testing.T) {
	pool := bancoComOAuth(t)

	for _, tabela := range tabelasOAuth {
		assert.Equal(t, 1, countRows(t, pool,
			"SELECT count(*) FROM pg_class WHERE relname = '"+tabela+"' AND relkind = 'r' AND NOT relrowsecurity"),
			"tabela %s deve existir sem RLS", tabela)
	}
}

// papeisSemAcessoOAuth são PUBLIC e todos os papéis criados pelas migrações.
var papeisSemAcessoOAuth = []string{"public", "markupp_app", "markupp_scaler", "markupp_indexer", "markupp_notifier"}

// bancoComOAuthSobDefaultPrivileges sobe até a 00031, concede DEFAULT PRIVILEGES a todos os
// papéis e só então aplica a 00032, simulando um banco que entrega tabelas novas a eles.
func bancoComOAuthSobDefaultPrivileges(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.UpTo(context.Background(), 31)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES
		TO PUBLIC, markupp_app, markupp_scaler, markupp_indexer, markupp_notifier`)
	require.NoError(t, err)
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
	return pool
}

// privilegiosOAuth conta os privilégios, de tabela ou de alguma coluna, que o papel tem na tabela.
func privilegiosOAuth(t *testing.T, pool *pgxpool.Pool, papel, tabela string) int {
	t.Helper()
	return countRows(t, pool, `SELECT count(*) FROM unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE',
		'TRUNCATE','REFERENCES','TRIGGER']) p WHERE has_table_privilege('`+papel+`', '`+tabela+`', p)
		OR (p IN ('SELECT','INSERT','UPDATE','REFERENCES') AND has_any_column_privilege('`+papel+`', '`+tabela+`', p))`)
}

func TestMigracao00032_TabelasOAuth_SemPrivilegioParaPapeis(t *testing.T) {
	pool := bancoComOAuthSobDefaultPrivileges(t)

	for _, papel := range papeisSemAcessoOAuth {
		for _, tabela := range tabelasOAuth {
			assert.Equal(t, 0, privilegiosOAuth(t, pool, papel, tabela), "%s não pode tocar em %s", papel, tabela)
		}
	}
}

func TestMigracao00032_ChavesEstrangeiras_Indexadas(t *testing.T) {
	pool := bancoComOAuth(t)

	for _, fk := range [][2]string{{"oauth_requests", "user_id"}, {"oauth_requests", "client_id"}, {"oauth_requests", "authorization_id"},
		{"oauth_authorizations", "user_id"}, {"oauth_authorizations", "client_id"}, {"oauth_tokens", "authorization_id"}} {
		assert.Positive(t, countRows(t, pool, `SELECT count(*) FROM pg_indexes WHERE tablename = '`+fk[0]+
			`' AND indexdef ~ '\(`+fk[1]+`[,)]'`), "%s.%s precisa de índice com a coluna na frente", fk[0], fk[1])
	}
}

func TestMigracao00032_EscopoInvalido_Rejeitado(t *testing.T) {
	pool := bancoComOAuth(t)

	assert.Error(t, autorizar(pool, "a1", "{notas:tudo}", false))
	assert.Error(t, autorizar(pool, "a2", "{}", false), "lista vazia de escopos")
	assert.NoError(t, autorizar(pool, "a3", "{notas:leitura,notas:escrita,administracao}", false))
}

func TestMigracao00032_SourceInvalido_Rejeitado(t *testing.T) {
	pool := bancoComOAuth(t)

	_, err := pool.Exec(context.Background(), `INSERT INTO oauth_clients (id, source, name, redirect_uris)
		VALUES ('c2', 'manual', 'Agente', ARRAY['http://127.0.0.1/cb'])`)

	assert.Error(t, err)
}

// O CHECK é o que garante que GetFreshCIMDClient nunca devolve cliente de outra origem:
// só o cimd tem validade de documento.
func TestMigracao00032_ValidadeDoDocumento_SoEmClienteCIMD(t *testing.T) {
	pool := bancoComOAuth(t)
	inserir := func(id, source string) error {
		_, err := pool.Exec(context.Background(), `INSERT INTO oauth_clients (id, source, name, redirect_uris, metadata_expires_at)
			VALUES ($1, $2, 'Agente', ARRAY['http://127.0.0.1/cb'], now() + interval '1 hour')`, id, source)
		return err
	}

	assert.Error(t, inserir("c2", "dcr"))
	assert.NoError(t, inserir("https://agente.example/c.json", "cimd"))
	_, err := pool.Exec(context.Background(), `UPDATE oauth_clients SET metadata_expires_at = now() WHERE id = 'c1'`)
	assert.Error(t, err)
}

func TestMigracao00032_DuasAutorizacoesAtivasDoMesmoPar_Rejeitadas(t *testing.T) {
	pool := bancoComOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", false))

	assert.Error(t, autorizar(pool, "a2", "{notas:leitura}", false))
}

func TestMigracao00032_AtivaMaisRevogadaDoMesmoPar_Aceitas(t *testing.T) {
	pool := bancoComOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", true))

	assert.NoError(t, autorizar(pool, "a2", "{notas:leitura}", false))
}

func TestMigracao00032_Down_RemoveTabelasOAuth(t *testing.T) {
	pool := bancoComOAuth(t)
	provider := gooseProvider(t, pool)

	_, err := provider.DownTo(context.Background(), 31)

	require.NoError(t, err)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM pg_class WHERE relname LIKE 'oauth\\_%'"))
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
}

func emitirToken(pool *pgxpool.Pool, hash, escopos string) error {
	_, err := pool.Exec(context.Background(), `INSERT INTO oauth_tokens (token_hash, authorization_id, kind, resource, scopes, expires_at)
		VALUES ($1, 'a1', 'access', 'https://mcp.markupp.dev', $2::TEXT[], now() + interval '1 hour')`, hash, escopos)
	return err
}

func TestMigracao00032_EscopoDoToken_RestritoAosSuportados(t *testing.T) {
	pool := bancoComOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura,administracao}", false))

	assert.NoError(t, emitirToken(pool, "t1", "{notas:leitura}"))
	assert.Error(t, emitirToken(pool, "t2", "{notas:tudo}"))
	assert.Error(t, emitirToken(pool, "t3", "{}"), "lista vazia de escopos")
}

func TestMigracao00032_TokenRotacionado_GuardaQuandoFoiTrocado(t *testing.T) {
	pool := bancoComOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", false))
	require.NoError(t, emitirToken(pool, "t1", "{notas:leitura}"))

	_, err := pool.Exec(context.Background(), `UPDATE oauth_tokens SET rotated_at = now() WHERE token_hash = 't1'`)

	assert.NoError(t, err)
}

// pedidoDaAutorizacao grava o pedido r1 de c1 ligado à Autorização authorizationID.
func pedidoDaAutorizacao(pool *pgxpool.Pool, authorizationID string) error {
	_, err := pool.Exec(context.Background(), `INSERT INTO oauth_requests (id, client_id, redirect_uri, scopes,
		resource, code_challenge, expires_at, authorization_id)
		VALUES ('r1', 'c1', 'http://127.0.0.1/cb', '{notas:leitura}', 'https://mcp.markupp.dev', 'desafio',
		now() + interval '10 minutes', $1)`, authorizationID)
	return err
}

func TestMigracao00032_PedidoLigadoAAutorizacao_SaiComEla(t *testing.T) {
	pool := bancoComOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", false))
	require.NoError(t, pedidoDaAutorizacao(pool, "a1"))

	_, err := pool.Exec(context.Background(), "DELETE FROM oauth_authorizations WHERE id = 'a1'")

	require.NoError(t, err)
	assert.Equal(t, 0, countRows(t, pool, "SELECT count(*) FROM oauth_requests"))
	assert.Error(t, pedidoDaAutorizacao(pool, "a-nenhuma"), "o pedido só aponta para Autorização existente")
}

func TestMigracao00032_PedidoGuardaQuandoOCodigoFoiReusado(t *testing.T) {
	pool := bancoComOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", false))
	require.NoError(t, pedidoDaAutorizacao(pool, "a1"))

	_, err := pool.Exec(context.Background(), "UPDATE oauth_requests SET code_reused_at = now() WHERE id = 'r1'")

	assert.NoError(t, err)
}
