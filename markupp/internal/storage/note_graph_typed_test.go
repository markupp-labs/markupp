package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// cofreTipado monta um cofre indexado com ligações tipadas: duas chaves para a mesma
// nota, chave junto de ligação do corpo, ligação tipada de volta, ligação tipada
// sem nota e frontmatter só com valores literais.
func cofreTipado(t *testing.T) *storage.PostgresNoteGraph {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("f", "Fulana.md", "---\nescreveu: \"[[p]]\"\n---\nautora"),
		notaComCaminho("p", "p.md", "---\nautor: \"[[Fulana]]\"\nrevisor: [\"[[fulana]]\"]\n---\nsem ligacao no corpo"),
		notaComCaminho("q", "q.md", "---\nautor: \"[[Fulana]]\"\n---\n[[Fulana]]"),
		notaComCaminho("o", "o.md", "---\ndata: 2026-10-19\ntags: [x]\n---\nso literais"),
		notaComCaminho("r", "r.md", "---\nrelacionada: \"[[Ninguem]]\"\n---\nx"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, "simple")
	return storage.NewPostgresNoteGraph(db)
}

func TestNoteGraph_Backlinks_TrazemOsPredicadosDasLigacoesTipadas(t *testing.T) {
	graph := cofreTipado(t)

	lidas, err := graph.Backlinks(ctxDeTeste, "f", 0, 100)

	require.NoError(t, err)
	require.Len(t, lidas, 2)
	assert.Equal(t, []notes.Backlink{
		{LinkedNote: notes.LinkedNote{ID: "p", VaultID: cofreDeTeste, Path: "p.md", UpdatedAt: lidas[0].UpdatedAt}, Predicates: []string{"autor", "revisor"}},
		{LinkedNote: notes.LinkedNote{ID: "q", VaultID: cofreDeTeste, Path: "q.md", UpdatedAt: lidas[1].UpdatedAt}, Predicates: []string{"autor"}},
	}, lidas, "uma linha por nota, com as chaves que a citam")
}

func TestNoteGraph_Neighbors_SeparamOsPredicadosPorSentido(t *testing.T) {
	graph := cofreTipado(t)

	vizinhos, err := graph.Neighbors(ctxDeTeste, "f", 0, 100)

	require.NoError(t, err)
	require.Len(t, vizinhos, 2)
	assert.Equal(t, notes.Neighbor{
		LinkedNote: notes.LinkedNote{ID: "p", VaultID: cofreDeTeste, Path: "p.md", UpdatedAt: vizinhos[0].UpdatedAt},
		Incoming:   true, Outgoing: true,
		IncomingPredicates: []string{"autor", "revisor"}, OutgoingPredicates: []string{"escreveu"},
	}, vizinhos[0])
	assert.Equal(t, []string{"autor"}, vizinhos[1].IncomingPredicates)
	assert.Equal(t, []string{}, vizinhos[1].OutgoingPredicates)
}

func TestNoteGraph_Orphans_LigacaoTipadaContaEValorLiteralNao(t *testing.T) {
	graph := cofreTipado(t)

	orfas, err := graph.Orphans(ctxDeTeste, cofreDeTeste, 0, 100)

	require.NoError(t, err)
	assert.Equal(t, []string{"o", "r"}, idsDe(orfas), "p so tem ligacao tipada e nao e orfa")
}

func TestNoteGraph_UnresolvedLinks_IncluiLigacaoTipadaSemNota(t *testing.T) {
	graph := cofreTipado(t)

	soltas, err := graph.UnresolvedLinks(ctxDeTeste, cofreDeTeste, 0, 100)

	require.NoError(t, err)
	assert.Equal(t, []notes.UnresolvedLink{{NoteID: "r", Path: "r.md", Title: "Ninguem", Kind: "typed"}}, soltas)
}

func TestVaultIndexer_GravaOPredicadoDaLigacaoTipada(t *testing.T) {
	db := setupNotesDB(t)
	storagetest.SaveNote(ctxDeTeste, t, storage.NewPostgresNotesRepository(db, "simple"),
		notaComCaminho("n1", "a.md", "---\nautor: \"[[Fulana]]\"\n---\n[[Fulana]]"))

	indexaTodosOsCofres(t, db, "simple")

	var ligacoes []string
	require.NoError(t, db.QueryRow(context.Background(), `SELECT array_agg(kind || '|' || coalesce(predicate, '') ORDER BY kind)
		FROM note_links WHERE note_id = 'n1'`).Scan(&ligacoes))
	assert.Equal(t, []string{"explicit|", "typed|autor"}, ligacoes)
}

func TestNoteLinks_PredicadoSoNaLigacaoTipada(t *testing.T) {
	db := setupNotesDB(t)
	storagetest.SaveNote(ctxDeTeste, t, storage.NewPostgresNotesRepository(db, "simple"), notaComCaminho("n1", "a.md", "x"))

	for _, sql := range []string{
		`INSERT INTO note_links (note_id, vault_id, cited_title, title_key, kind, predicate) VALUES ('n1', 'cofre-teste', 'B', 'b', 'explicit', 'autor')`,
		`INSERT INTO note_links (note_id, vault_id, cited_title, title_key, kind) VALUES ('n1', 'cofre-teste', 'B', 'b', 'typed')`,
	} {
		_, err := db.Exec(context.Background(), sql)
		require.Error(t, err, sql)
	}
}

func TestMigracao00022_ReenfileiraTodasAsNotasParaDerivarAsLigacoesTipadas(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.UpTo(context.Background(), 21)
	require.NoError(t, err)
	storagetest.SeedMember(t, pool, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	_, err = pool.Exec(context.Background(), `INSERT INTO notes (id, tenant_id, vault_id, path, content, created_at, updated_at)
		VALUES ('a', 't1', 'va', 'a.md', '---\nautor: "[[b]]"\n---\n', now(), now())`)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), "DELETE FROM note_index_queue")
	require.NoError(t, err)

	_, err = provider.UpTo(context.Background(), 22)

	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, pool, consultaDoScaler), "notas ja indexadas voltam para a fila para ganhar ligacoes tipadas")
}

func TestMigracao00022_DownEUp(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.Up(context.Background())
	require.NoError(t, err)

	_, err = provider.DownTo(context.Background(), 21)
	require.NoError(t, err)
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
}
