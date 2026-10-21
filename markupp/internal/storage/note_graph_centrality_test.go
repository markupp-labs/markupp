package storage_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// cofreComDoisGrupos monta um cofre indexado com dois triângulos de notas ligados por
// uma ponte de c para d, a nota indice citada por a, b e c e, por erro de digitação
// que vira ligação inferida, por d, e a nota solta.
func cofreComDoisGrupos(t *testing.T) (*pgxpool.Pool, *storage.PostgresNotesRepository, *storage.PostgresNoteGraph) {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("a", "a.md", "[[b]] [[indice]]"),
		notaComCaminho("b", "b.md", "[[c]] [[indice]]"),
		notaComCaminho("c", "c.md", "[[a]] [[indice]] [[d]]"),
		notaComCaminho("d", "d.md", "[[e]] [[indise]]"),
		notaComCaminho("e", "e.md", "[[f]]"),
		notaComCaminho("f", "f.md", "[[d]]"),
		notaComCaminho("indice", "indice.md", "sem ligacao de saida"),
		notaComCaminho("solta", "solta.md", "nada"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, "simple")
	return db, repo, storage.NewPostgresNoteGraph(db, "simple")
}

// comunidadesLidas lê as comunidades do cofre e as notas de cada uma, por id.
func comunidadesLidas(t *testing.T, repo *storage.PostgresNotesRepository, graph *storage.PostgresNoteGraph) map[int64][]string {
	t.Helper()
	comunidades, err := graph.Communities(ctxDeTeste, cofreDeTeste, notes.Page{Limit: 100})
	require.NoError(t, err)
	grupos := map[int64][]string{}
	for _, c := range comunidades {
		membros := idsDasNotas(notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Community: &c.ID, Limit: 100}))
		assert.Len(t, membros, c.Size, "comunidade %d", c.ID)
		slices.Sort(membros)
		grupos[c.ID] = membros
	}
	return grupos
}

// idDaComunidade devolve o id da comunidade que tem a nota noteID.
func idDaComunidade(grupos map[int64][]string, noteID string) int64 {
	for id, membros := range grupos {
		if slices.Contains(membros, noteID) {
			return id
		}
	}
	return 0
}

func TestCommunities_SeparaOsGruposESemNotaSolta(t *testing.T) {
	_, repo, graph := cofreComDoisGrupos(t)

	grupos := comunidadesLidas(t, repo, graph)

	assert.ElementsMatch(t, [][]string{{"a", "b", "c", "indice"}, {"d", "e", "f"}}, slices.Collect(maps.Values(grupos)))
}

func TestCommunities_DaMaiorParaAMenorComANotaDeMaiorPageRank(t *testing.T) {
	_, _, graph := cofreComDoisGrupos(t)

	comunidades, err := graph.Communities(ctxDeTeste, cofreDeTeste, notes.Page{Limit: 100})

	require.NoError(t, err)
	require.Len(t, comunidades, 2)
	assert.Equal(t, []int{4, 3}, []int{comunidades[0].Size, comunidades[1].Size})
	assert.Equal(t, resumo("indice", "indice.md"), comunidades[0].Hub)
	pagina, err := graph.Communities(ctxDeTeste, cofreDeTeste, notes.Page{Offset: 1, Limit: 1})
	require.NoError(t, err)
	assert.Equal(t, comunidades[1:], pagina)
}

func TestCommunity_LeUmaPeloIdEDizQuandoNaoExiste(t *testing.T) {
	_, _, graph := cofreComDoisGrupos(t)
	comunidades, err := graph.Communities(ctxDeTeste, cofreDeTeste, notes.Page{Limit: 100})
	require.NoError(t, err)

	lida, err := graph.Community(ctxDeTeste, cofreDeTeste, comunidades[1].ID)
	require.NoError(t, err)
	assert.Equal(t, comunidades[1], lida)
	_, err = graph.Community(ctxDeTeste, cofreDeTeste, -1)
	require.ErrorIs(t, err, notes.ErrCommunityNotFound)
}

func TestCommunities_ComunidadeQueCresceHerdaOId(t *testing.T) {
	db, repo, graph := cofreComDoisGrupos(t)
	antes := comunidadesLidas(t, repo, graph)
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("g", "g.md", "[[d]] [[e]] [[f]]"))

	indexaTodosOsCofres(t, db, "simple")

	depois := comunidadesLidas(t, repo, graph)
	assert.Equal(t, idDaComunidade(antes, "a"), idDaComunidade(depois, "a"))
	assert.Equal(t, idDaComunidade(antes, "d"), idDaComunidade(depois, "g"), "d, e, f e g dividem 3/4 com d, e, f")
	assert.Equal(t, []string{"d", "e", "f", "g"}, depois[idDaComunidade(depois, "g")])
}

func TestCommunities_CofreInvisivel_ErrVaultForbidden(t *testing.T) {
	db, _, graph := cofreComDoisGrupos(t)
	estranho := identity.Actor{UserID: "u-fora", TenantID: "outro", Role: "admin"}
	ctxFora := storagetest.SeedMember(t, db, estranho, "cofre-fora")

	_, err := graph.Communities(ctxFora, cofreDeTeste, notes.Page{Limit: 10})
	require.ErrorIs(t, err, notes.ErrVaultForbidden)
	_, err = graph.Community(ctxFora, cofreDeTeste, 1)
	require.ErrorIs(t, err, notes.ErrVaultForbidden)
}

func TestCommunities_ViewerLe(t *testing.T) {
	db, _, graph := cofreComDoisGrupos(t)
	viewer := identity.Actor{UserID: "u-viewer", TenantID: membroDeTeste.TenantID, Role: "viewer"}
	ctxViewer := storagetest.SeedMember(t, db, viewer, cofreDeTeste)

	comunidades, err := graph.Communities(ctxViewer, cofreDeTeste, notes.Page{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, comunidades, 2)
}

func TestRLS_PapelDoIndexador_NaoGravaCentralidadeNemComunidadeDeOutroCofre(t *testing.T) {
	db := setupTestDB(t)
	ator := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, ator, "va")
	storagetest.SeedMember(t, db, ator, "vb")

	for _, sql := range []string{
		"WITH i AS (INSERT INTO note_centrality (note_id, vault_id, pagerank, in_degree, out_degree) VALUES ('x', 'vb', 1, 0, 0) RETURNING 1) SELECT count(*) FROM i",
		"WITH i AS (INSERT INTO note_communities (note_id, vault_id, community_id) VALUES ('x', 'vb', 1) RETURNING 1) SELECT count(*) FROM i",
	} {
		_, err := comoIndexador(t, db, "t1", "va", sql)

		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr, sql)
		assert.Equal(t, "42501", pgErr.Code, "recebido %v", err)
	}
}
