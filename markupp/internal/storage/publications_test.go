package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

var instantePublicado = time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)

func contextoDe(ator identity.Actor) context.Context {
	return identity.WithActor(context.Background(), ator)
}

func rascunho(id, vaultID string, alvo publication.Target, ator identity.Actor) publication.Draft {
	return publication.Draft{ID: id, VaultID: vaultID, Target: alvo, PublishedBy: ator.UserID, PublishedAt: instantePublicado}
}

func eventoDe(id, action string, ator identity.Actor) auth.AuditEventRecord {
	return auth.AuditEventRecord{ID: "ev-" + id + action, TenantID: ator.TenantID, ActorID: ator.UserID, Action: action, Target: id, Result: "success", OccurredAt: instantePublicado}
}

func repositorioDePublicacoes(t *testing.T) (*pgxpool.Pool, *storage.PostgresPublications) {
	t.Helper()
	db := tenantComDoisCofres(t)
	return db, storage.NewPostgresPublications(db)
}

func TestAlvo_NotaPastaECofre_NomeEContagem(t *testing.T) {
	_, repo := repositorioDePublicacoes(t)
	casos := map[publication.Target]publication.TargetInfo{
		{Kind: publication.KindNote, NoteID: "na"}:         {Name: "na", NoteCount: 1},
		{Kind: publication.KindFolder, FolderPath: "docs"}: {Name: "docs", NoteCount: 1},
		{Kind: publication.KindVault}:                      {Name: "va", NoteCount: 1},
	}
	for alvo, esperado := range casos {
		info, err := repo.TargetInfo(contextoDe(membroDoCofre), "va", alvo)

		require.NoError(t, err, alvo.Kind)
		assert.Equal(t, esperado, info, alvo.Kind)
	}
}

func TestAlvo_InexistenteOuForaDoAlcance_DevolveOErroDoAlvo(t *testing.T) {
	_, repo := repositorioDePublicacoes(t)
	casos := []struct {
		cofre string
		alvo  publication.Target
		erro  error
	}{
		{"va", publication.Target{Kind: publication.KindNote, NoteID: "nb"}, publication.ErrNoteNotFound},
		{"va", publication.Target{Kind: publication.KindNote, NoteID: "nenhuma"}, publication.ErrNoteNotFound},
		{"va", publication.Target{Kind: publication.KindFolder, FolderPath: "doc"}, publication.ErrFolderNotFound},
		{"vb", publication.Target{Kind: publication.KindVault}, notes.ErrVaultForbidden},
	}
	for _, c := range casos {
		_, err := repo.TargetInfo(contextoDe(membroDoCofre), c.cofre, c.alvo)

		require.ErrorIs(t, err, c.erro, "%+v", c.alvo)
	}
}

func TestInserir_NotaPeloMembro_GravaAuditaEDevolveOIDPublico(t *testing.T) {
	db, repo := repositorioDePublicacoes(t)

	p, err := repo.InsertTx(contextoDe(membroDoCofre), rascunho("p1", "va", publication.Target{Kind: publication.KindNote, NoteID: "na"}, membroDoCofre), eventoDe("p1", publication.ActionPublish, membroDoCofre))

	require.NoError(t, err)
	assert.Regexp(t, formatoDoIDPublico, p.PublicID)
	p.PublicID, p.PublishedAt = "", p.PublishedAt.UTC()
	assert.Equal(t, publication.Publication{
		ID: "p1", VaultID: "va", VaultName: "va", Kind: "note", NoteID: "na", TargetName: "na", NoteCount: 1,
		PublishedByID: "membro", PublishedByName: "membro@markupp.dev", PublishedAt: instantePublicado,
	}, p)
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM audit_events WHERE action = 'publications.publish' AND target = 'p1'"))
}

func TestInserir_AlvoJaPublicado_DevolveAPublicacaoExistente(t *testing.T) {
	db, repo := repositorioDePublicacoes(t)
	alvo := publication.Target{Kind: publication.KindFolder, FolderPath: "docs"}
	_, err := repo.InsertTx(contextoDe(donaDoTenant), rascunho("p1", "va", alvo, donaDoTenant), eventoDe("p1", publication.ActionPublish, donaDoTenant))
	require.NoError(t, err)

	_, err = repo.InsertTx(contextoDe(donaDoTenant), rascunho("p2", "va", alvo, donaDoTenant), eventoDe("p2", publication.ActionPublish, donaDoTenant))

	var jaPublicado *publication.AlreadyPublishedError
	require.ErrorAs(t, err, &jaPublicado)
	assert.Equal(t, "p1", jaPublicado.PublicationID)
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM audit_events"), "falha nao audita")
}

func TestInserir_PastaPeloMembro_RecusadaPeloBanco(t *testing.T) {
	db, repo := repositorioDePublicacoes(t)

	_, err := repo.InsertTx(contextoDe(membroDoCofre), rascunho("p1", "va", publication.Target{Kind: publication.KindFolder, FolderPath: "docs"}, membroDoCofre), eventoDe("p1", publication.ActionPublish, membroDoCofre))

	require.ErrorIs(t, err, publication.ErrPublishForbidden)
	assert.Contains(t, err.Error(), `"p1"`)
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM publications"))
}

func TestListar_AdminForaDoCofreVeTodasComNomesEFiltraPorCofre(t *testing.T) {
	db, repo := repositorioDePublicacoes(t)
	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))
	publica(t, db, "p-nb", "t1", "vb", alvoNota("nb"))

	todas, err := repo.List(contextoDe(adminDeFora), "")
	require.NoError(t, err)
	doCofre, err := repo.List(contextoDe(adminDeFora), "va")
	require.NoError(t, err)

	assert.Len(t, todas, 2)
	require.Len(t, doCofre, 1)
	assert.Equal(t, []string{"p-docs", "va", "docs", "docs"}, []string{doCofre[0].ID, doCofre[0].VaultName, doCofre[0].FolderPath, doCofre[0].TargetName})
	assert.Empty(t, doCofre[0].PublishedByName, "quem publicou nao e conta do tenant")
}

func TestIndexacao_TrocaAuditaEDistingueInexistenteDeProibido(t *testing.T) {
	db, repo := repositorioDePublicacoes(t)
	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))

	p, err := repo.SetIndexableTx(contextoDe(donaDoTenant), "p-docs", true, eventoDe("p-docs", publication.ActionUpdate, donaDoTenant))
	require.NoError(t, err)
	assert.True(t, p.Indexable)
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM audit_events WHERE action = 'publications.update'"))

	_, err = repo.SetIndexableTx(contextoDe(membroDoCofre), "p-docs", false, eventoDe("p-docs", publication.ActionUpdate, membroDoCofre))
	require.ErrorIs(t, err, publication.ErrPublishForbidden)
	_, err = repo.SetIndexableTx(contextoDe(donoDeOutro), "p-docs", false, eventoDe("p-docs", publication.ActionUpdate, donoDeOutro))
	require.ErrorIs(t, err, publication.ErrPublicationNotFound)
	assert.Contains(t, err.Error(), `"p-docs"`)
}

func TestDespublicar_ApagaAuditaEDistingueInexistenteDeProibido(t *testing.T) {
	db, repo := repositorioDePublicacoes(t)
	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))

	require.ErrorIs(t, repo.DeleteTx(contextoDe(membroDoCofre), "p-docs", eventoDe("p-docs", publication.ActionUnpublish, membroDoCofre)), publication.ErrPublishForbidden)
	require.ErrorIs(t, repo.DeleteTx(contextoDe(donaDoTenant), "nenhuma", eventoDe("nenhuma", publication.ActionUnpublish, donaDoTenant)), publication.ErrPublicationNotFound)
	require.NoError(t, repo.DeleteTx(contextoDe(adminDeFora), "p-docs", eventoDe("p-docs", publication.ActionUnpublish, adminDeFora)))

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM publications"))
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM public_notes"))
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM audit_events WHERE action = 'publications.unpublish'"))
}
