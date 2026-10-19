package storage_test

import (
	"cmp"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// notasDoCofre lê a página das notas de cofreDeTeste pedida por q.
func notasDoCofre(ctx context.Context, t *testing.T, repo *storage.PostgresNotesRepository, q notes.VaultNotesQuery) []notes.Note {
	t.Helper()
	lidas, err := repo.ListVaultNotes(ctx, cofreDeTeste, q)
	require.NoError(t, err, "notas do cofre com %+v", q)
	return lidas
}

func idsDasNotas(lidas []notes.Note) []string {
	ids := make([]string, 0, len(lidas))
	for _, lida := range lidas {
		ids = append(ids, lida.ID)
	}
	return ids
}

func ptr[T any](v T) *T { return &v }

func TestListVaultNotes_SemOrdemPedida_OrdenaPorCaminho(t *testing.T) {
	_, repo, _ := cofreLigado(t)

	lidas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Limit: 100})

	assert.Equal(t, []string{"a", "c", "d", "e", "f", "g", "c2", "b"}, idsDasNotas(lidas))
	assert.Equal(t, "[[B]] [[Inexistente]] ![[foto.png]]", lidas[0].Content, "a nota vem inteira")
}

func TestListVaultNotes_TrazGrauPageRankEComunidadeEOrdenaPorPageRank(t *testing.T) {
	_, repo, _ := cofreComDoisGrupos(t)

	lidas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Sort: notes.SortByPageRank, Descending: true, Limit: 100})

	require.Len(t, lidas, 8, "toda nota do cofre tem centralidade")
	assert.Equal(t, "indice", lidas[0].ID)
	assert.Equal(t, 4, *lidas[0].Rank.Degree, "d cita indice por inferencia")
	assert.NotNil(t, lidas[0].Rank.Community)
	assert.True(t, slices.IsSortedFunc(lidas, func(x, y notes.Note) int { return cmp.Compare(*y.Rank.PageRank, *x.Rank.PageRank) }))
	total := 0.0
	for _, n := range lidas {
		total += *n.Rank.PageRank
	}
	assert.InDelta(t, 1, total, 1e-6)
	solta := lidas[slices.IndexFunc(lidas, func(n notes.Note) bool { return n.ID == "solta" })]
	assert.Equal(t, notes.NoteRank{Degree: ptr(0), PageRank: solta.Rank.PageRank}, solta.Rank, "nota sozinha nao tem comunidade")
}

func TestListVaultNotes_Paginadas(t *testing.T) {
	_, repo, _ := cofreComDoisGrupos(t)

	lidas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Sort: notes.SortByPageRank, Descending: true, Offset: 1, Limit: 2})

	require.Len(t, lidas, 2)
	assert.NotEqual(t, "indice", lidas[0].ID)
}

func TestListVaultNotes_OrdenaPorGrauEAtualizacaoNosDoisSentidos(t *testing.T) {
	_, repo, _ := cofreComDoisGrupos(t)
	_, err := repo.Update(ctxDeTeste, "e", "e.md", "[[f]]", time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC), time.Time{}, true)
	require.NoError(t, err)

	porGrau := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Sort: notes.SortByDegree, Limit: 100})
	recentes := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Sort: notes.SortByUpdated, Descending: true, Limit: 100})
	antigas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Sort: notes.SortByUpdated, Limit: 100})

	assert.Equal(t, "solta", porGrau[0].ID, "grau crescente")
	assert.True(t, slices.IsSortedFunc(porGrau, func(x, y notes.Note) int { return cmp.Compare(*x.Rank.Degree, *y.Rank.Degree) }))
	assert.Equal(t, "e", recentes[0].ID)
	assert.Equal(t, "e", antigas[len(antigas)-1].ID)
}

func TestListVaultNotes_Orfas_GrauZeroGravadoPeloSegundoEstagio(t *testing.T) {
	_, repo, _ := cofreLigado(t)
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("nova", "nova.md", "sem indexar"))

	orfas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Orphan: ptr(true), Limit: 100})
	ligadas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Orphan: ptr(false), Limit: 100})
	todas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Limit: 100})

	assert.Equal(t, []string{"e", "g"}, idsDasNotas(orfas), "ligacao para si mesma nao conta; a nota nova so entra depois do segundo estagio")
	assert.Equal(t, []string{"a", "c", "d", "f", "c2", "b"}, idsDasNotas(ligadas))
	nova := todas[slices.IndexFunc(todas, func(n notes.Note) bool { return n.ID == "nova" })]
	assert.Equal(t, notes.NoteRank{}, nova.Rank, "sem segundo estagio, sem organizacao derivada")
}

func TestListVaultNotes_LigacaoInferidaContaParaNotaOrfa(t *testing.T) {
	_, repo, _ := cofreComErroDeDigitacao(t)

	orfas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Orphan: ptr(true), Limit: 100})

	assert.Equal(t, []string{"m", "p"}, idsDasNotas(orfas))
}

func TestListVaultNotes_PorComunidade(t *testing.T) {
	_, repo, _ := cofreComDoisGrupos(t)
	indice, err := repo.GetNoteByID(ctxDeTeste, "indice")
	require.NoError(t, err)

	lidas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Community: indice.Rank.Community, Sort: notes.SortByPageRank, Descending: true, Limit: 100})

	assert.ElementsMatch(t, []string{"a", "b", "c", "indice"}, idsDasNotas(lidas))
	assert.Equal(t, "indice", lidas[0].ID)
	assert.Empty(t, notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Community: ptr(int64(-1)), Limit: 100}))
}

func TestListVaultNotes_CofreInvisivel_ErrVaultForbidden(t *testing.T) {
	db, repo, _ := cofreLigado(t)
	estranho := identity.Actor{UserID: "u-fora", TenantID: "outro", Role: "admin"}
	ctxFora := storagetest.SeedMember(t, db, estranho, "cofre-fora")

	_, err := repo.ListVaultNotes(ctxFora, cofreDeTeste, notes.VaultNotesQuery{Limit: 10})

	require.ErrorIs(t, err, notes.ErrVaultForbidden)
}

func TestListVaultNotes_ViewerLe(t *testing.T) {
	db, repo, _ := cofreLigado(t)
	viewer := identity.Actor{UserID: "u-viewer", TenantID: membroDeTeste.TenantID, Role: "viewer"}
	ctxViewer := storagetest.SeedMember(t, db, viewer, cofreDeTeste)

	orfas := notasDoCofre(ctxViewer, t, repo, notes.VaultNotesQuery{Orphan: ptr(true), Limit: 10})

	assert.Equal(t, []string{"e", "g"}, idsDasNotas(orfas))
}

func TestGetNoteByIDEUpdate_TrazemAOrganizacaoDerivada(t *testing.T) {
	_, repo, _ := cofreComDoisGrupos(t)

	indice, err := repo.GetNoteByID(ctxDeTeste, "indice")
	require.NoError(t, err)
	solta, err := repo.Update(ctxDeTeste, "solta", "solta.md", "ainda nada", time.Now(), time.Time{}, true)
	require.NoError(t, err)

	assert.Equal(t, 4, *indice.Rank.Degree)
	assert.Positive(t, *indice.Rank.PageRank)
	assert.NotNil(t, indice.Rank.Community)
	assert.Equal(t, 0, *solta.Rank.Degree, "a escrita devolve o que o ultimo segundo estagio gravou")
	assert.Nil(t, solta.Rank.Community)
}

func TestGetNoteByID_AntesDoSegundoEstagio_SemOrganizacaoDerivada(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("a", "a.md", "x"))

	lida, err := repo.GetNoteByID(ctxDeTeste, "a")

	require.NoError(t, err)
	assert.Equal(t, notes.NoteRank{}, lida.Rank)
}
