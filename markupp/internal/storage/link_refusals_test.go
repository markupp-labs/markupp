package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/linkrefusals"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

var primeiraPagina = notes.Page{Limit: 100}

func TestRecusa_TiraALigacaoNaHoraEAOrfaVoltaNoSegundoEstagio(t *testing.T) {
	db, repo, graph := cofreComLigacaoPorMencao(t)
	recusas := storage.NewPostgresLinkRefusals(db)

	recusa, err := recusas.Create(ctxDeTeste, cofreDeTeste, "pulumi", "pipeline")
	require.NoError(t, err)

	assert.Equal(t, [2]notes.NoteSummary{resumo("pipeline", "Pipeline.md"), resumo("pulumi", "Pulumi.md")}, recusa.Notes,
		"o par nao tem sentido: as notas saem na ordem do id")
	saida, err := graph.Links(ctxDeTeste, "pipeline", todas)
	require.NoError(t, err)
	assert.Empty(t, saida, "a leitura ja respeita a recusa")
	indexaTodosOsCofres(t, db, "simple")
	orfas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Orphan: ptr(true), Limit: 100})
	assert.Equal(t, []string{"pipeline", "solta"}, idsDasNotas(orfas), "a recusa marca o cofre para o segundo estagio")
}

func TestRecusa_AparecemNaMencaoNaoLigada(t *testing.T) {
	db, _, graph := cofreComLigacaoPorMencao(t)
	_, err := storage.NewPostgresLinkRefusals(db).Create(ctxDeTeste, cofreDeTeste, "pipeline", "pulumi")
	require.NoError(t, err)

	mencoes, err := graph.Mentions(ctxDeTeste, "pipeline", primeiraPagina)

	require.NoError(t, err)
	require.Len(t, mencoes, 1)
	assert.Equal(t, resumo("pulumi", "Pulumi.md"), mencoes[0].Note)
}

func TestRecusa_RepetidaDevolveAMesma(t *testing.T) {
	db, _, _ := cofreComLigacaoPorMencao(t)
	recusas := storage.NewPostgresLinkRefusals(db)

	primeira, err := recusas.Create(ctxDeTeste, cofreDeTeste, "pipeline", "pulumi")
	require.NoError(t, err)
	segunda, err := recusas.Create(ctxDeTeste, cofreDeTeste, "pulumi", "pipeline")
	require.NoError(t, err)

	assert.Equal(t, primeira, segunda)
	lista, err := recusas.List(ctxDeTeste, cofreDeTeste, primeiraPagina)
	require.NoError(t, err)
	assert.Equal(t, []linkrefusals.Refusal{primeira}, lista)
}

func TestRecusa_DesfeitaDevolveALigacao(t *testing.T) {
	db, _, graph := cofreComLigacaoPorMencao(t)
	recusas := storage.NewPostgresLinkRefusals(db)
	recusa, err := recusas.Create(ctxDeTeste, cofreDeTeste, "pipeline", "pulumi")
	require.NoError(t, err)

	require.NoError(t, recusas.Delete(ctxDeTeste, cofreDeTeste, recusa.ID))

	saida, err := graph.Links(ctxDeTeste, "pipeline", todas)
	require.NoError(t, err)
	assert.Len(t, saida, 1)
	require.ErrorIs(t, recusas.Delete(ctxDeTeste, cofreDeTeste, recusa.ID), linkrefusals.ErrRefusalNotFound)
}

func TestRecusa_NotaDeFora_ErrNoteOutsideVault(t *testing.T) {
	db, repo, _ := cofreComLigacaoPorMencao(t)
	outro := identity.Actor{UserID: membroDeTeste.UserID, TenantID: membroDeTeste.TenantID, Role: membroDeTeste.Role}
	storagetest.SeedMember(t, db, outro, "outro-cofre")
	nota := notaComCaminho("fora", "Fora.md", "x")
	nota.VaultID = "outro-cofre"
	storagetest.SaveNote(ctxDeTeste, t, repo, nota)

	_, err := storage.NewPostgresLinkRefusals(db).Create(ctxDeTeste, cofreDeTeste, "pipeline", "fora")

	require.ErrorIs(t, err, linkrefusals.ErrNoteOutsideVault)
}

func TestRecusa_CofreInvisivel_ErrVaultForbidden(t *testing.T) {
	db, _, _ := cofreComLigacaoPorMencao(t)
	estranho := identity.WithActor(ctxDeTeste, identity.Actor{UserID: "u-fora", TenantID: membroDeTeste.TenantID, Role: "member"})

	_, err := storage.NewPostgresLinkRefusals(db).List(estranho, cofreDeTeste, primeiraPagina)

	require.ErrorIs(t, err, notes.ErrVaultForbidden)
}

func TestRLS_ViewerNaoGravaRecusa(t *testing.T) {
	db, _, _ := cofreComLigacaoPorMencao(t)
	viewer := identity.Actor{UserID: "u-viewer", TenantID: membroDeTeste.TenantID, Role: "viewer"}
	ctxViewer := storagetest.SeedMember(t, db, viewer, cofreDeTeste)

	_, err := storage.NewPostgresLinkRefusals(db).Create(ctxViewer, cofreDeTeste, "pipeline", "pulumi")

	require.Error(t, err, "a politica barra o viewer mesmo que o servico deixe passar")
}
