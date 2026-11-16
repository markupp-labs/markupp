package publication_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
)

var instanteDaPublicacao = time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)

// fakePublicationRepository responde o alvo configurado e guarda o que o serviço gravou.
type fakePublicationRepository struct {
	info      publication.TargetInfo
	infoErr   error
	inserted  *publication.Draft
	events    []auth.AuditEventRecord
	indexable *bool
	deleted   string
}

func (f *fakePublicationRepository) TargetInfo(_ context.Context, _ string, _ publication.Target) (publication.TargetInfo, error) {
	return f.info, f.infoErr
}

func (f *fakePublicationRepository) InsertTx(_ context.Context, d publication.Draft, e auth.AuditEventRecord) (publication.Publication, error) {
	f.inserted, f.events = &d, append(f.events, e)
	return publication.Publication{ID: d.ID, Kind: d.Target.Kind}, nil
}

func (f *fakePublicationRepository) List(_ context.Context, _ string) ([]publication.Publication, error) {
	return []publication.Publication{{ID: "p1"}}, nil
}

func (f *fakePublicationRepository) SetIndexableTx(_ context.Context, id string, indexable bool, e auth.AuditEventRecord) (publication.Publication, error) {
	f.indexable, f.events = &indexable, append(f.events, e)
	return publication.Publication{ID: id, Indexable: indexable}, nil
}

func (f *fakePublicationRepository) DeleteTx(_ context.Context, id string, e auth.AuditEventRecord) error {
	f.deleted, f.events = id, append(f.events, e)
	return nil
}

func servicoCom(repo *fakePublicationRepository) *publication.Service {
	return publication.NewService(repo, func() time.Time { return instanteDaPublicacao }, func() string { return "id-novo" })
}

var (
	dona   = identity.Actor{UserID: "dona", TenantID: "t1", Role: identity.RoleOwner}
	membro = identity.Actor{UserID: "membro", TenantID: "t1", Role: identity.RoleMember}
	viewer = identity.Actor{UserID: "viewer", TenantID: "t1", Role: identity.RoleViewer}
)

func pedido(alvo publication.Target, confirmacao string) publication.Request {
	return publication.Request{Target: alvo, Confirm: confirmacao, Indexable: true}
}

func TestPublicar_ConfirmacaoCerta_GravaComAuditoria(t *testing.T) {
	repo := &fakePublicationRepository{info: publication.TargetInfo{Name: "docs/deploy", NoteCount: 3}}

	p, err := servicoCom(repo).Publish(context.Background(), dona, "va", pedido(publication.Target{Kind: publication.KindFolder, FolderPath: "docs/deploy"}, "docs/deploy"))

	require.NoError(t, err)
	assert.Equal(t, "id-novo", p.ID)
	assert.Equal(t, publication.Draft{
		ID: "id-novo", VaultID: "va", Target: publication.Target{Kind: publication.KindFolder, FolderPath: "docs/deploy"},
		Indexable: true, PublishedBy: "dona", PublishedAt: instanteDaPublicacao,
	}, *repo.inserted)
	assert.Equal(t, auth.AuditEventRecord{
		ID: "id-novo", TenantID: "t1", ActorID: "dona", Action: "publications.publish",
		Target: "id-novo", Result: "success", OccurredAt: instanteDaPublicacao,
	}, repo.events[0])
}

func TestPublicar_ConfirmacaoErrada_RecusaCitandoOQueVeioEONomeEsperado(t *testing.T) {
	repo := &fakePublicationRepository{info: publication.TargetInfo{Name: "Guia de deploy"}}

	_, err := servicoCom(repo).Publish(context.Background(), membro, "va", pedido(publication.Target{Kind: publication.KindNote, NoteID: "n1"}, "guia"))

	require.ErrorIs(t, err, publication.ErrConfirmationMismatch)
	assert.Contains(t, err.Error(), `"guia"`)
	assert.Contains(t, err.Error(), `"Guia de deploy"`)
	assert.Nil(t, repo.inserted)
}

func TestPublicar_AlvoForaDoFormato_RecusaComOValorEOFormato(t *testing.T) {
	invalidos := []publication.Target{
		{Kind: "pagina"},
		{Kind: publication.KindNote},
		{Kind: publication.KindFolder, FolderPath: "/docs"},
		{Kind: publication.KindFolder, FolderPath: "docs/"},
		{Kind: publication.KindFolder},
	}
	for _, alvo := range invalidos {
		_, err := servicoCom(&fakePublicationRepository{}).Publish(context.Background(), dona, "va", pedido(alvo, "x"))

		require.ErrorIs(t, err, publication.ErrInvalidTarget, "%+v", alvo)
		assert.Contains(t, err.Error(), "expected", "%+v", alvo)
	}
}

func TestPublicar_PapelSemPermissao_RecusaAntesDeLerOAlvo(t *testing.T) {
	recusas := []struct {
		ator identity.Actor
		alvo publication.Target
	}{
		{viewer, publication.Target{Kind: publication.KindNote, NoteID: "n1"}},
		{membro, publication.Target{Kind: publication.KindFolder, FolderPath: "docs"}},
		{membro, publication.Target{Kind: publication.KindVault}},
	}
	for _, r := range recusas {
		repo := &fakePublicationRepository{infoErr: publication.ErrNoteNotFound}

		_, err := servicoCom(repo).Publish(context.Background(), r.ator, "va", pedido(r.alvo, "x"))

		require.ErrorIs(t, err, publication.ErrPublishForbidden, "%s em %s", r.ator.Role, r.alvo.Kind)
		assert.Contains(t, err.Error(), r.ator.Role)
	}
}

func TestPublicar_AlvoInexistente_DevolveOErroDoRepositorio(t *testing.T) {
	repo := &fakePublicationRepository{infoErr: publication.ErrFolderNotFound}

	_, err := servicoCom(repo).Publish(context.Background(), dona, "va", pedido(publication.Target{Kind: publication.KindFolder, FolderPath: "vazia"}, "vazia"))

	require.ErrorIs(t, err, publication.ErrFolderNotFound)
}

func TestPrevia_PastaECofreIncluemAsNotasFuturas(t *testing.T) {
	repo := &fakePublicationRepository{info: publication.TargetInfo{Name: "Cofre", NoteCount: 7}}
	for kind, futuras := range map[string]bool{publication.KindNote: false, publication.KindFolder: true, publication.KindVault: true} {
		alvo := publication.Target{Kind: kind, NoteID: "n1", FolderPath: "docs"}

		previa, err := servicoCom(repo).Preview(context.Background(), "va", alvo)

		require.NoError(t, err)
		assert.Equal(t, publication.Preview{Kind: kind, TargetName: "Cofre", NoteCount: 7, IncludesFuture: futuras}, previa)
	}
}

func TestIndexacao_TrocaEAudita(t *testing.T) {
	repo := &fakePublicationRepository{}

	p, err := servicoCom(repo).SetIndexable(context.Background(), membro, "p1", true)

	require.NoError(t, err)
	assert.True(t, p.Indexable)
	assert.Equal(t, "publications.update", repo.events[0].Action)
	assert.Equal(t, "p1", repo.events[0].Target)
}

func TestDespublicar_ApagaEAudita(t *testing.T) {
	repo := &fakePublicationRepository{}

	require.NoError(t, servicoCom(repo).Unpublish(context.Background(), dona, "p1"))

	assert.Equal(t, "p1", repo.deleted)
	assert.Equal(t, "publications.unpublish", repo.events[0].Action)
}

func TestDespublicarOuIndexar_Viewer_RecusadoSemTocarNoRepositorio(t *testing.T) {
	repo := &fakePublicationRepository{}
	svc := servicoCom(repo)

	errDespublicar := svc.Unpublish(context.Background(), viewer, "p1")
	_, errIndexar := svc.SetIndexable(context.Background(), viewer, "p1", true)

	require.ErrorIs(t, errDespublicar, publication.ErrPublishForbidden)
	require.ErrorIs(t, errIndexar, publication.ErrPublishForbidden)
	assert.Empty(t, repo.events)
}

func TestJaPublicado_CarregaOIDDaPublicacaoExistente(t *testing.T) {
	err := error(&publication.AlreadyPublishedError{PublicationID: "p-velha"})

	require.ErrorIs(t, err, publication.ErrAlreadyPublished)
	assert.Contains(t, err.Error(), `"p-velha"`)
}
