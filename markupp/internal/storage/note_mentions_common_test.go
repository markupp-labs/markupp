package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// cofreComTituloComum monta um cofre indexado de 110 notas em que "projeto", de uma
// palavra, e "plano diretor", de duas, aparecem em sete corpos, mais de 5% do cofre,
// e "kubernetes" só no corpo de m.
func cofreComTituloComum(t *testing.T) (*pgxpool.Pool, *storage.PostgresNoteGraph) {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("proj", "Projeto.md", "x"),
		notaComCaminho("pd", "Plano Diretor.md", "x"),
		notaComCaminho("k", "Kubernetes.md", "x"),
		notaComCaminho("m", "m.md", "o projeto segue o plano diretor no kubernetes"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	semeiaNotas(t, db, "u", "o projeto segue o plano diretor", 6)
	semeiaNotas(t, db, "e", "enchimento", enchimento)
	indexaTodosOsCofres(t, db, "simple")
	return db, storage.NewPostgresNoteGraph(db, "simple")
}

func TestNoteGraph_Mentions_TituloEmMaisDe5PorCentoDoCofreNaoEMencao(t *testing.T) {
	_, graph := cofreComTituloComum(t)

	assert.Equal(t, []string{"Kubernetes.md kubernetes"}, resumoDasMencoes(mencoes(t, graph, "mentions", "m", 0, 100)),
		"titulo comum e palavra, nao entidade (Mihalcea e Csomai, 2007), de uma palavra ou de varias")
	assert.Empty(t, mencoes(t, graph, "backmentions", "proj", 0, 100), "projeto e comum pelo df")
	assert.Empty(t, mencoes(t, graph, "backmentions", "pd", 0, 100), "plano diretor e comum pela contagem")
	assert.Equal(t, []string{"m.md kubernetes"}, resumoDasMencoes(mencoes(t, graph, "backmentions", "k", 0, 100)))
}

func TestNoteGraph_Mentions_CofreSemDfNaoFiltraTituloComum(t *testing.T) {
	db, graph := cofreComTituloComum(t)
	_, err := db.Exec(context.Background(), "DELETE FROM vault_term_stats WHERE vault_id = $1", cofreDeTeste)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"Projeto.md projeto", "Plano Diretor.md plano diretor", "Kubernetes.md kubernetes"},
		resumoDasMencoes(mencoes(t, graph, "mentions", "m", 0, 100)),
		"sem o N do cofre nao ha como julgar a frequencia: na duvida, a mencao fica")
	assert.Len(t, mencoes(t, graph, "backmentions", "proj", 0, 100), 7)
}
