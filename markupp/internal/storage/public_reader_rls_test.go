package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

const papelDoLeitor = "markupp_reader"

// leituraDoLeitor são as relações em que o papel da leitura pública tem SELECT, e só
// SELECT: as da nota e as do plano de recuperação que as rotas públicas leem.
var leituraDoLeitor = map[string]bool{
	"notes": true, "vault_publications": true, "note_search_index": true, "note_titles": true,
	"note_links": true, "note_centrality": true, "note_communities": true, "vault_graph_versions": true,
	"note_edges": true, "note_link_items": true, "note_unresolved_link_items": true,
}

// privilegiosDoLeitor lista os privilégios, de tabela ou de alguma coluna, que o leitor
// tem em cada tabela e view do schema public.
func privilegiosDoLeitor(t *testing.T, db *pgxpool.Pool) map[string][]string {
	t.Helper()
	rows, err := db.Query(context.Background(), `SELECT c.relname, p
		FROM pg_class c JOIN pg_namespace s ON s.oid = c.relnamespace
		CROSS JOIN unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER']) p
		WHERE s.nspname = 'public' AND c.relkind IN ('r', 'v', 'm', 'p')
		  AND (has_table_privilege($1, c.oid, p)
		       OR (p IN ('SELECT','INSERT','UPDATE','REFERENCES') AND has_any_column_privilege($1, c.oid, p)))`, papelDoLeitor)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var relacao, privilegio string
		require.NoError(t, rows.Scan(&relacao, &privilegio))
		out[relacao] = append(out[relacao], privilegio)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestMigracao00036_Leitor_SoLeAsTabelasDaNotaEDaRecuperacao(t *testing.T) {
	db := setupTestDB(t)

	privilegios := privilegiosDoLeitor(t, db)

	for relacao, lista := range privilegios {
		assert.True(t, leituraDoLeitor[relacao], "leitor nao pode tocar em %s, tem %v", relacao, lista)
		assert.Equal(t, []string{"SELECT"}, lista, "leitor so le %s", relacao)
	}
	for relacao := range leituraDoLeitor {
		assert.Contains(t, privilegios, relacao, "leitor precisa ler %s", relacao)
	}
}

func TestMigracao00036_Leitor_NaoLeQuemPublicouNemOTenant(t *testing.T) {
	db := setupTestDB(t)

	for _, coluna := range []string{"tenant_id", "published_by"} {
		var permitido bool
		require.NoError(t, db.QueryRow(context.Background(),
			"SELECT has_column_privilege($1, 'vault_publications', $2, 'SELECT')", papelDoLeitor, coluna).Scan(&permitido))
		assert.False(t, permitido, "leitor le vault_publications.%s", coluna)
	}
}

// publicaCofre grava, com o papel dono do banco, a publicação do cofre vaultID.
func publicaCofre(t *testing.T, db *pgxpool.Pool, tenantID, vaultID, slug string) {
	t.Helper()
	_, err := db.Exec(context.Background(), `INSERT INTO vault_publications (vault_id, tenant_id, slug, title, published_by, published_at)
		VALUES ($1, $2, $3, $3, 'quem-publicou', now())`, vaultID, tenantID, slug)
	require.NoError(t, err, "publicar o cofre %q como %q", vaultID, slug)
}

// doisCofresUmPublicado semeia o cofre publicado "pub", com duas notas, e o privado
// "priv", com uma, em tenants diferentes, e indexa os dois.
func doisCofresUmPublicado(t *testing.T) *pgxpool.Pool {
	t.Helper()
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctxPub := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "owner"}, "pub")
	ctxPriv := storagetest.SeedMember(t, db, identity.Actor{UserID: "ub", TenantID: "t2", Role: "owner"}, "priv")
	storagetest.SaveNote(ctxPub, t, repo, notaPublica("p1", "guia.md", "pub", "leia o [[inicio]] antes do guia"))
	storagetest.SaveNote(ctxPub, t, repo, notaPublica("p2", "inicio.md", "pub", "primeiros passos do guia"))
	storagetest.SaveNote(ctxPriv, t, repo, notaPublica("s1", "segredo.md", "priv", "guia secreto, cita [[inicio]]"))
	indexaTodosOsCofres(t, db, "simple")
	publicaCofre(t, db, "t1", "pub", "docs")
	return db
}

func notaPublica(id, path, vaultID, content string) notes.Note {
	agora := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	return notes.Note{ID: id, VaultID: vaultID, Path: path, Content: content, CreatedAt: agora, UpdatedAt: agora}
}

// comoLeitor conta, no papel da leitura pública, as linhas que sql devolve.
func comoLeitor(t *testing.T, db *pgxpool.Pool, sql string) (int64, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "SELECT set_config('role', $1, true)", papelDoLeitor)
	require.NoError(t, err)
	var n int64
	err = tx.QueryRow(ctx, sql).Scan(&n)
	return n, err
}

func TestRLS_LeitorSemFiltro_SoEnxergaOsCofresPublicados(t *testing.T) {
	db := doisCofresUmPublicado(t)

	consultas := map[string]int64{
		"SELECT count(*) FROM notes":                                         2,
		"SELECT count(*) FROM note_titles":                                   2,
		"SELECT count(*) FROM note_search_index":                             2,
		"SELECT count(*) FROM vault_publications":                            1,
		"SELECT count(*) FROM notes WHERE vault_id = 'priv'":                 0,
		"SELECT count(*) FROM note_search_index WHERE note_id = 's1'":        0,
		"SELECT count(*) FROM vault_graph_versions WHERE vault_id = 'priv'":  0,
		"SELECT count(*) FROM note_link_items WHERE vault_id = 'priv'":       0,
		"SELECT count(*) FROM note_centrality WHERE vault_id = 'priv'":       0,
		"SELECT count(*) FROM vault_graph_versions WHERE vault_id = 'pub'":   1,
		"SELECT count(*) FROM note_centrality WHERE vault_id = 'pub'":        2,
		"SELECT count(*) FROM notes WHERE vault_id = 'pub' AND id IN ('p1')": 1,
	}
	for sql, esperado := range consultas {
		n, err := comoLeitor(t, db, sql)
		require.NoError(t, err, sql)
		assert.Equal(t, esperado, n, sql)
	}
}

func TestRLS_LeitorDepoisDeDespublicar_NaoEnxergaNada(t *testing.T) {
	db := doisCofresUmPublicado(t)
	_, err := db.Exec(context.Background(), "DELETE FROM vault_publications WHERE vault_id = 'pub'")
	require.NoError(t, err)

	n, err := comoLeitor(t, db, "SELECT count(*) FROM notes")

	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestRLS_Leitor_NaoEscreveEmNada(t *testing.T) {
	db := doisCofresUmPublicado(t)

	escritas := []string{
		"WITH i AS (INSERT INTO notes (id, tenant_id, vault_id, path, content, created_at, updated_at) VALUES ('x', 't1', 'pub', 'x.md', '', now(), now()) RETURNING 1) SELECT count(*) FROM i",
		"WITH u AS (UPDATE notes SET content = 'pichado' WHERE id = 'p1' RETURNING 1) SELECT count(*) FROM u",
		"WITH d AS (DELETE FROM notes WHERE id = 'p1' RETURNING 1) SELECT count(*) FROM d",
		"WITH u AS (UPDATE vault_publications SET slug = 'outro' RETURNING 1) SELECT count(*) FROM u",
		"WITH d AS (DELETE FROM vault_publications RETURNING 1) SELECT count(*) FROM d",
		"WITH i AS (INSERT INTO vault_publications (vault_id, tenant_id, slug, title, published_by, published_at) VALUES ('priv', 't2', 'vaza', 'v', 'x', now()) RETURNING 1) SELECT count(*) FROM i",
		"WITH d AS (DELETE FROM note_search_index RETURNING 1) SELECT count(*) FROM d",
	}
	for _, sql := range escritas {
		_, err := comoLeitor(t, db, sql)
		requirePermissaoNegada(t, err, sql)
	}
}

func TestRLS_Leitor_NaoLeContasNemTenants(t *testing.T) {
	db := doisCofresUmPublicado(t)

	for _, tabela := range []string{"users", "tenants", "tenant_memberships", "vaults", "vault_members", "user_invites", "audit_events", "notifications", "note_terms"} {
		_, err := comoLeitor(t, db, "SELECT count(*) FROM "+tabela)
		requirePermissaoNegada(t, err, "leitor lendo "+tabela)
	}
}

func TestRLS_PapelDaAplicacaoComoMembro_NaoPublica(t *testing.T) {
	db := setupTestDB(t)
	membro := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, membro, "va")

	_, err := comoPapelDaAplicacao(t, db, membro, `WITH i AS (INSERT INTO vault_publications (vault_id, slug, title, published_by, published_at)
		VALUES ('va', 'meu-cofre', 'x', 'ua', now()) RETURNING 1) SELECT count(*) FROM i`)

	requirePermissaoNegada(t, err, "membro publicando")
}
