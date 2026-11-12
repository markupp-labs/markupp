package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

const papelDoLeitor = "markupp_reader"

// leituraDoLeitor são as relações em que o papel da leitura pública tem SELECT, e só
// SELECT: as da nota e da organização derivada que as rotas públicas leem.
var leituraDoLeitor = map[string]bool{
	"notes": true, "public_notes": true, "note_search_index": true, "note_titles": true,
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

func TestMigracao00040_Leitor_SoLeAsTabelasDaNotaEDaOrganizacao(t *testing.T) {
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

func notaPublica(id, path, vaultID, content string) notes.Note {
	agora := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	return notes.Note{ID: id, VaultID: vaultID, Path: path, Content: content, CreatedAt: agora, UpdatedAt: agora}
}

// notaPublicadaEPrivadas semeia no cofre "pub" a nota publicada p1, que cita a vizinha
// privada p2, e no cofre "priv", de outro tenant, a nota s1; indexa tudo e publica p1.
func notaPublicadaEPrivadas(t *testing.T) *pgxpool.Pool {
	t.Helper()
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctxPub := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "owner"}, "pub")
	ctxPriv := storagetest.SeedMember(t, db, identity.Actor{UserID: "ub", TenantID: "t2", Role: "owner"}, "priv")
	storagetest.SaveNote(ctxPub, t, repo, notaPublica("p1", "guia.md", "pub", "leia o [[inicio]] antes do guia"))
	storagetest.SaveNote(ctxPub, t, repo, notaPublica("p2", "inicio.md", "pub", "primeiros passos, cita o [[guia]]"))
	storagetest.SaveNote(ctxPriv, t, repo, notaPublica("s1", "segredo.md", "priv", "guia secreto, cita [[inicio]]"))
	indexaTodosOsCofres(t, db, "simple")
	publica(t, db, "pub-p1", "t1", "pub", alvoNota("p1"))
	return db
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

func TestRLS_LeitorSemFiltro_SoEnxergaAsNotasPublicadas(t *testing.T) {
	db := notaPublicadaEPrivadas(t)

	consultas := map[string]int64{
		"SELECT count(*) FROM notes":                                        1,
		"SELECT count(*) FROM notes WHERE id IN ('p2', 's1')":               0,
		"SELECT count(*) FROM note_titles":                                  1,
		"SELECT count(*) FROM note_search_index":                            1,
		"SELECT count(*) FROM note_links":                                   1,
		"SELECT count(*) FROM note_centrality":                              1,
		"SELECT count(*) FROM note_communities WHERE note_id <> 'p1'":       0,
		"SELECT count(*) FROM public_notes":                                 1,
		"SELECT count(*) FROM note_edges":                                   0,
		"SELECT count(*) FROM note_link_items WHERE target_id IS NOT NULL":  0,
		"SELECT count(*) FROM vault_graph_versions WHERE vault_id = 'priv'": 0,
	}
	for sql, esperado := range consultas {
		n, err := comoLeitor(t, db, sql)
		require.NoError(t, err, sql)
		assert.Equal(t, esperado, n, sql)
	}
}

func TestRLS_LeitorDepoisDeDespublicar_NaoEnxergaNada(t *testing.T) {
	db := notaPublicadaEPrivadas(t)
	despublica(t, db, "pub-p1")

	for _, tabela := range []string{"notes", "public_notes", "note_search_index", "note_links"} {
		n, err := comoLeitor(t, db, "SELECT count(*) FROM "+tabela)
		require.NoError(t, err, tabela)
		assert.Zero(t, n, tabela)
	}
}

func TestRLS_Leitor_NaoEscreveEmNada(t *testing.T) {
	db := notaPublicadaEPrivadas(t)

	escritas := []string{
		"WITH i AS (INSERT INTO notes (id, tenant_id, vault_id, path, content, created_at, updated_at) VALUES ('x', 't1', 'pub', 'x.md', '', now(), now()) RETURNING 1) SELECT count(*) FROM i",
		"WITH u AS (UPDATE notes SET content = 'pichado' WHERE id = 'p1' RETURNING 1) SELECT count(*) FROM u",
		"WITH d AS (DELETE FROM notes WHERE id = 'p1' RETURNING 1) SELECT count(*) FROM d",
		"WITH i AS (INSERT INTO public_notes (note_id, public_id, vault_id, publication_id) VALUES ('s1', 'AAAAAAAAAAAA', 'priv', 'pub-p1') RETURNING 1) SELECT count(*) FROM i",
		"WITH u AS (UPDATE public_notes SET note_id = 's1' RETURNING 1) SELECT count(*) FROM u",
		"WITH d AS (DELETE FROM public_notes RETURNING 1) SELECT count(*) FROM d",
		"WITH i AS (INSERT INTO publications (id, tenant_id, vault_id, kind, published_by, published_at) VALUES ('x', 't2', 'priv', 'vault', 'x', now()) RETURNING 1) SELECT count(*) FROM i",
		"WITH d AS (DELETE FROM note_search_index RETURNING 1) SELECT count(*) FROM d",
	}
	for _, sql := range escritas {
		_, err := comoLeitor(t, db, sql)
		requirePermissaoNegada(t, err, sql)
	}
}

func TestRLS_Leitor_NaoLeContasTenantsNemPublicacoes(t *testing.T) {
	db := notaPublicadaEPrivadas(t)

	for _, tabela := range []string{"users", "tenants", "tenant_memberships", "vaults", "vault_members", "user_invites", "audit_events", "notifications", "note_terms", "publications"} {
		_, err := comoLeitor(t, db, "SELECT count(*) FROM "+tabela)
		requirePermissaoNegada(t, err, "leitor lendo "+tabela)
	}
}

func TestRLS_Leitor_EscopoPublicoSoDeIDPublicoExistente(t *testing.T) {
	db := notaPublicadaEPrivadas(t)

	n, err := comoLeitor(t, db, "SELECT count(*) FROM markupp_public_scope((SELECT public_id FROM public_notes))")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	n, err = comoLeitor(t, db, "SELECT count(*) FROM markupp_public_scope('AAAAAAAAAAAA')")
	require.NoError(t, err)
	assert.Zero(t, n)
	_, err = comoLeitor(t, db, "SELECT count(*) FROM markupp_list_publications('', '')")
	requirePermissaoNegada(t, err, "leitor listando publicacoes")
}

// poolComSessaoDoDono abre sobre o banco de db um pool cujas conexões já trazem, no
// nível da sessão, o tenant, o usuário e o papel do dono do cofre privado "priv".
func poolComSessaoDoDono(t *testing.T, db *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	cfg := db.Config()
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SELECT set_config('markupp.tenant_id', 't2', false),
			set_config('markupp.user_id', 'ub', false), set_config('markupp.role', 'owner', false)`)
		return err
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// Regressão: as políticas antigas de notes valem para todos os papéis, e a transação
// pública herdaria da sessão o tenant e o usuário de quem usou a conexão antes.
func TestRLS_LeitorComTenantEUsuarioNaSessao_SoEnxergaAsNotasPublicadas(t *testing.T) {
	pool := poolComSessaoDoDono(t, notaPublicadaEPrivadas(t))
	ctx := context.Background()

	err := storage.InPublicTx(ctx, pool, func(q *gen.Queries) error {
		_, err := q.GetNoteByID(ctx, "s1")
		return err
	})

	require.ErrorIs(t, err, pgx.ErrNoRows, "leitor publico leu a nota do cofre privado")
}
