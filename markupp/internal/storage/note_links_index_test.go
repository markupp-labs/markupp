package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// linhasDoGrafo lê, com o papel dono do banco, as ligações e os títulos gravados
// para a nota noteID, em texto para comparar.
func linhasDoGrafo(t *testing.T, db *pgxpool.Pool, noteID string) (ligacoes, titulos []string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, db.QueryRow(ctx, `SELECT coalesce(array_agg(cited_title || '|' || title_key || '|' || coalesce(target_path, '') || '|' || kind
		|| '|' || tenant_id || '|' || vault_id ORDER BY title_key, kind), '{}') FROM note_links WHERE note_id = $1`, noteID).Scan(&ligacoes))
	require.NoError(t, db.QueryRow(ctx, `SELECT coalesce(array_agg(title_key || '|' || tenant_id || '|' || vault_id ORDER BY title_key), '{}')
		FROM note_titles WHERE note_id = $1`, noteID).Scan(&titulos))
	return ligacoes, titulos
}

func TestVaultIndexer_GravaLigacoesETitulosDaNota(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n1", "pasta/Origem.md",
		"---\naliases: [Apelido]\n---\n[[Alvo]] ![[foto.png]] [md](Irma.md)"))

	indexaTodosOsCofres(t, db, "simple")

	ligacoes, titulos := linhasDoGrafo(t, db, "n1")
	assert.Equal(t, []string{
		"Alvo|alvo||explicit|default|cofre-teste",
		"foto.png|foto.png||embed|default|cofre-teste",
		"Irma|irma|pasta/Irma.md|explicit|default|cofre-teste",
	}, ligacoes)
	assert.Equal(t, []string{"apelido|default|cofre-teste", "origem|default|cofre-teste"}, titulos)
}

func TestVaultIndexer_NotaReindexada_SubstituiLigacoesETitulos(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n1", "velha.md", "[[Antigo]]"))
	indexaTodosOsCofres(t, db, "simple")

	_, err := repo.Update(ctxDeTeste, "n1", "nova.md", "[[Novo]]", time.Now(), time.Time{}, true)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	ligacoes, titulos := linhasDoGrafo(t, db, "n1")
	assert.Equal(t, []string{"Novo|novo||explicit|default|cofre-teste"}, ligacoes)
	assert.Equal(t, []string{"nova|default|cofre-teste"}, titulos)
}

func TestVaultIndexer_NotaApagada_LevaLigacoesETitulos(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n1", "a.md", "[[Alvo]]"))
	indexaTodosOsCofres(t, db, "simple")

	require.NoError(t, repo.Delete(ctxDeTeste, "n1"))

	ligacoes, titulos := linhasDoGrafo(t, db, "n1")
	assert.Empty(t, ligacoes)
	assert.Empty(t, titulos)
}

func TestSearchNotes_AliasPesaComoTitulo(t *testing.T) {
	repo := cofreBuscavel(t, "simple",
		notaComCaminho("corpo", "x.md", "fala de kubectl"),
		notaComCaminho("alias", "Kubernetes.md", "---\naliases: [kubectl]\n---\noutro assunto"))

	assert.Equal(t, []string{"alias", "corpo"}, idsDaBusca(ctxDeTeste, t, repo, "kubectl"))
}

func TestRLS_LigacoesETitulos_LidosPorQuemLeANotaInclusiveViewer(t *testing.T) {
	db := setupNotesDB(t)
	storagetest.SaveNote(ctxDeTeste, t, storage.NewPostgresNotesRepository(db, "simple"), notaComCaminho("n1", "a.md", "[[Alvo]]"))
	indexaTodosOsCofres(t, db, "simple")
	viewer := identity.Actor{UserID: "u-viewer", TenantID: membroDeTeste.TenantID, Role: "viewer"}
	storagetest.SeedMember(t, db, viewer, cofreDeTeste)
	estranho := identity.Actor{UserID: "u-fora", TenantID: "outro", Role: "admin"}
	storagetest.SeedMember(t, db, estranho, "cofre-fora")

	for _, sql := range []string{"SELECT count(*) FROM note_links", "SELECT count(*) FROM note_titles"} {
		n, err := comoPapelDaAplicacao(t, db, viewer, sql)
		require.NoError(t, err, sql)
		assert.Equal(t, int64(1), n, "viewer do cofre le: %s", sql)
		n, err = comoPapelDaAplicacao(t, db, estranho, sql)
		require.NoError(t, err, sql)
		assert.Zero(t, n, "outro tenant nao le: %s", sql)
	}
}

func TestRLS_LigacoesETitulos_AplicacaoNaoEscreve(t *testing.T) {
	db := setupNotesDB(t)
	storagetest.SaveNote(ctxDeTeste, t, storage.NewPostgresNotesRepository(db, "simple"), notaComCaminho("n1", "a.md", "[[Alvo]]"))
	indexaTodosOsCofres(t, db, "simple")

	for _, sql := range []string{
		"WITH d AS (DELETE FROM note_links RETURNING 1) SELECT count(*) FROM d",
		"WITH d AS (DELETE FROM note_titles RETURNING 1) SELECT count(*) FROM d",
	} {
		_, err := comoPapelDaAplicacao(t, db, membroDeTeste, sql)
		requirePermissaoNegada(t, err, sql)
	}
}

func TestRLS_LigacoesETitulos_IndexadorSoEnxergaOCofreDaSessao(t *testing.T) {
	db := setupNotesDB(t)
	storagetest.SaveNote(ctxDeTeste, t, storage.NewPostgresNotesRepository(db, "simple"), notaComCaminho("n1", "a.md", "[[Alvo]]"))
	indexaTodosOsCofres(t, db, "simple")

	for _, sql := range []string{"SELECT count(*) FROM note_links", "SELECT count(*) FROM note_titles"} {
		n, err := comoIndexador(t, db, membroDeTeste.TenantID, cofreDeTeste, sql)
		require.NoError(t, err, sql)
		assert.Equal(t, int64(1), n, sql)
		n, err = comoIndexador(t, db, membroDeTeste.TenantID, "outro-cofre", sql)
		require.NoError(t, err, sql)
		assert.Zero(t, n, sql)
	}
}

func TestMigracao00021_ReenfileiraTodasAsNotasParaDerivarAsLigacoes(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.UpTo(context.Background(), 20)
	require.NoError(t, err)
	storagetest.SeedMember(t, pool, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	_, err = pool.Exec(context.Background(), `INSERT INTO notes (id, tenant_id, vault_id, path, content, created_at, updated_at)
		VALUES ('a', 't1', 'va', 'a.md', '[[b]]', now(), now())`)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), "DELETE FROM note_index_queue")
	require.NoError(t, err)

	_, err = provider.UpTo(context.Background(), 21)

	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, pool, consultaDoScaler), "notas ja indexadas voltam para a fila para ganhar ligacoes")
}

func TestMigracao00021_DownEUp(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.Up(context.Background())
	require.NoError(t, err)

	_, err = provider.DownTo(context.Background(), 20)
	require.NoError(t, err)
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
}
