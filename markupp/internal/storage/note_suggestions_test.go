package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// cofreComSugestoes monta um cofre indexado em que x e y ligam a z, lex fala do mesmo
// assunto que x sem ligação, l já liga a x e longe não tem nada em comum com x.
func cofreComSugestoes(t *testing.T) *storage.PostgresNoteGraph {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("x", "x.md", "[[z]] kubernetes orquestra conteineres"),
		notaComCaminho("y", "y.md", "[[z]]"),
		notaComCaminho("z", "z.md", "alvo"),
		notaComCaminho("l", "l.md", "[[x]] kubernetes orquestra conteineres"),
		notaComCaminho("lex", "lex.md", "kubernetes orquestra conteineres"),
		notaComCaminho("longe", "longe.md", "nada a ver"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, "simple")
	return storage.NewPostgresNoteGraph(db, "simple")
}

func sugestoes(t *testing.T, graph *storage.PostgresNoteGraph, noteID string, offset, limit int) []notes.LinkSuggestion {
	t.Helper()
	lidas, err := graph.Suggestions(ctxDeTeste, noteID, notes.Page{Offset: offset, Limit: limit})
	require.NoError(t, err, "sugestoes de %q", noteID)
	return lidas
}

func caminhosDasSugestoes(lidas []notes.LinkSuggestion) []string {
	caminhos := make([]string, 0, len(lidas))
	for _, lida := range lidas {
		caminhos = append(caminhos, lida.Note.Path)
	}
	return caminhos
}

func TestNoteGraph_Suggestions_FundeEstruturaETextoSemAPropriaNemAsLigadas(t *testing.T) {
	graph := cofreComSugestoes(t)

	lidas := sugestoes(t, graph, "x", 0, 100)

	assert.Equal(t, []string{"y.md", "lex.md"}, caminhosDasSugestoes(lidas),
		"y vem nas duas listas; z e l ja se ligam a x")
	require.Len(t, lidas, 2)
	assert.InDelta(t, 0.5, lidas[0].ResourceAllocation, 1e-9, "z tem dois vizinhos, x e y")
	assert.InDelta(t, 1.0/61+1.0/62, lidas[0].Score, 1e-9, "primeira na estrutural e segunda na lexica")
	assert.Zero(t, lidas[1].ResourceAllocation)
	assert.Greater(t, lidas[1].Cosine, lidas[0].Cosine, "lex divide mais termos com x do que y")
	assert.Equal(t, resumo("lex", "lex.md"), lidas[1].Note)
}

func TestNoteGraph_Suggestions_Paginado(t *testing.T) {
	graph := cofreComSugestoes(t)

	assert.Equal(t, []string{"lex.md"}, caminhosDasSugestoes(sugestoes(t, graph, "x", 1, 1)))
}

func TestNoteGraph_Suggestions_NotaSemLigacaoNemTermoEmComum_Vazio(t *testing.T) {
	graph := cofreComSugestoes(t)

	assert.Empty(t, sugestoes(t, graph, "longe", 0, 100))
}

func TestNoteGraph_Suggestions_CandidataLexicaSoPelosDezTermosDeMaiorPeso(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	repetidas := palavrasDistintas("raro", 10)
	for _, nota := range []notes.Note{
		notaComCaminho("x", "x.md", repetidas+" "+repetidas+" fraco"),
		notaComCaminho("c", "c.md", "fraco"),
		notaComCaminho("o1", "o1.md", "outro"),
		notaComCaminho("o2", "o2.md", "outro"),
		notaComCaminho("o3", "o3.md", "outro"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, "simple")
	graph := storage.NewPostgresNoteGraph(db, "simple")

	assert.Empty(t, sugestoes(t, graph, "x", 0, 100), "fraco e o 12o termo de x: c nao e candidata")
	assert.Equal(t, []string{"x.md"}, caminhosDasSugestoes(sugestoes(t, graph, "c", 0, 100)), "fraco e o termo de maior peso de c")
}

func TestNoteGraph_Suggestions_ViewerLeEEstranhoNaoVe(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("a", "a.md", "kubernetes orquestra"))
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("b", "b.md", "kubernetes orquestra"))
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("c", "c.md", "outro assunto"))
	indexaTodosOsCofres(t, db, "simple")
	graph := storage.NewPostgresNoteGraph(db, "simple")
	viewer := identity.Actor{UserID: "u-viewer", TenantID: membroDeTeste.TenantID, Role: "viewer"}
	ctxViewer := storagetest.SeedMember(t, db, viewer, cofreDeTeste)
	estranho := identity.Actor{UserID: "u-fora", TenantID: "outro", Role: "admin"}
	ctxFora := storagetest.SeedMember(t, db, estranho, "cofre-fora")

	lidas, err := graph.Suggestions(ctxViewer, "a", notes.Page{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"b.md"}, caminhosDasSugestoes(lidas))
	_, err = graph.Suggestions(ctxFora, "a", notes.Page{Limit: 10})
	require.ErrorIs(t, err, notes.ErrNotFound)
}

// semeiaNotas grava, com o papel dono do banco e num comando só, n notas em
// cofreDeTeste com o mesmo conteúdo, de ids prefixo0001 em diante e caminhos
// prefixo0001.md em diante. Os triggers as põem na fila do indexador.
func semeiaNotas(t *testing.T, db *pgxpool.Pool, prefixo, conteudo string, n int) {
	t.Helper()
	_, err := db.Exec(context.Background(), `INSERT INTO notes (id, tenant_id, vault_id, path, content, created_at, updated_at)
SELECT $1 || lpad(i::text, 4, '0'), $2, $3, $1 || lpad(i::text, 4, '0') || '.md', $4, now(), now()
FROM generate_series(1, $5::int) AS i`, prefixo, membroDeTeste.TenantID, cofreDeTeste, conteudo, n)
	require.NoError(t, err, "semear %d notas %q no cofre %q", n, prefixo, cofreDeTeste)
}

func TestNoteGraph_Suggestions_ListaLexicaAteAsMilPrimeiras(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("x", "x.md", "alfa"))
	semeiaNotas(t, db, "c", "alfa", 1001)
	semeiaNotas(t, db, "o", "beta", 200) // alfa fora de algumas notas, para ter idf
	indexaTodosOsCofres(t, db, "simple")
	graph := storage.NewPostgresNoteGraph(db, "simple")

	assert.Equal(t, []string{"c1000.md"}, caminhosDasSugestoes(sugestoes(t, graph, "x", 999, 100)),
		"o RRF funde as mil primeiras de cada lista, como Cormack, Clarke e Buttcher (2009); a 1001a fica de fora")
}
