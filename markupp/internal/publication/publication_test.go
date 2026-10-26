package publication_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
)

// fakePublicationRepository guarda o que o serviço pediu, sem banco.
type fakePublicationRepository struct {
	published   publication.Publication
	unpublished string
	events      []auth.AuditEventRecord
	stored      publication.Publication
	err         error
}

func (f *fakePublicationRepository) PublishTx(_ context.Context, p publication.Publication, event auth.AuditEventRecord) (publication.Publication, error) {
	f.published = p
	f.events = append(f.events, event)
	return p, f.err
}

func (f *fakePublicationRepository) UnpublishTx(_ context.Context, vaultID string, event auth.AuditEventRecord) error {
	f.unpublished = vaultID
	f.events = append(f.events, event)
	return f.err
}

func (f *fakePublicationRepository) Publication(_ context.Context, _ string) (publication.Publication, error) {
	return f.stored, f.err
}

var (
	agoraDaPublicacao = time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	adminDoTenant     = identity.Actor{UserID: "u-admin", TenantID: "t1", Role: identity.RoleAdmin}
)

func novoServico(repo *fakePublicationRepository) *publication.Service {
	return publication.NewService(repo, func() time.Time { return agoraDaPublicacao })
}

func TestPublicar_Admin_GravaComAuditoria(t *testing.T) {
	repo := &fakePublicationRepository{}

	p, err := novoServico(repo).Publish(context.Background(), adminDoTenant, "va", "docs", "  Documentacao  ")

	require.NoError(t, err)
	assert.Equal(t, publication.Publication{VaultID: "va", Slug: "docs", Title: "Documentacao", PublishedBy: "u-admin", PublishedAt: agoraDaPublicacao}, p)
	require.Len(t, repo.events, 1)
	evento := repo.events[0]
	assert.NotEmpty(t, evento.ID)
	evento.ID = ""
	assert.Equal(t, auth.AuditEventRecord{TenantID: "t1", ActorID: "u-admin", Action: "vaults.publish", Target: "va", Result: "success", OccurredAt: agoraDaPublicacao}, evento)
}

func TestPublicar_MembroOuViewer_Recusado(t *testing.T) {
	for _, papel := range []string{identity.RoleMember, identity.RoleViewer} {
		repo := &fakePublicationRepository{}

		_, err := novoServico(repo).Publish(context.Background(), identity.Actor{UserID: "u", TenantID: "t1", Role: papel}, "va", "docs", "")

		require.ErrorIs(t, err, publication.ErrNotAdmin, papel)
		assert.Contains(t, err.Error(), papel)
		assert.Empty(t, repo.events, "%s nao chega ao banco", papel)
	}
}

func TestPublicar_SlugForaDoFormato_RecusadoComOValor(t *testing.T) {
	for _, slug := range []string{"", "ab", "Docs", "com espaco", "acento-é", "a_b", strings.Repeat("a", 64)} {
		repo := &fakePublicationRepository{}

		_, err := novoServico(repo).Publish(context.Background(), adminDoTenant, "va", slug, "")

		require.ErrorIs(t, err, publication.ErrInvalidSlug, slug)
		assert.Contains(t, err.Error(), `"`+slug+`"`)
		assert.Contains(t, err.Error(), "a-z, 0-9 e hifen")
		assert.Empty(t, repo.events)
	}
}

func TestPublicar_SlugNosLimites_Aceito(t *testing.T) {
	for _, slug := range []string{"abc", "docs-v2", strings.Repeat("a", 63)} {
		_, err := novoServico(&fakePublicationRepository{}).Publish(context.Background(), adminDoTenant, "va", slug, "")

		require.NoError(t, err, slug)
	}
}

func TestPublicar_TituloLongoDemais_Recusado(t *testing.T) {
	titulo := strings.Repeat("é", publication.MaxTitleLength+1)

	_, err := novoServico(&fakePublicationRepository{}).Publish(context.Background(), adminDoTenant, "va", "docs", titulo)

	require.ErrorIs(t, err, publication.ErrInvalidTitle)
	assert.Contains(t, err.Error(), "201")
}

func TestDespublicar_Admin_AuditaADespublicacao(t *testing.T) {
	repo := &fakePublicationRepository{}

	err := novoServico(repo).Unpublish(context.Background(), adminDoTenant, "va")

	require.NoError(t, err)
	assert.Equal(t, "va", repo.unpublished)
	require.Len(t, repo.events, 1)
	assert.Equal(t, "vaults.unpublish", repo.events[0].Action)
	assert.Equal(t, "va", repo.events[0].Target)
}

func TestDespublicar_Viewer_Recusado(t *testing.T) {
	repo := &fakePublicationRepository{}

	err := novoServico(repo).Unpublish(context.Background(), identity.Actor{UserID: "u", TenantID: "t1", Role: identity.RoleViewer}, "va")

	require.ErrorIs(t, err, publication.ErrNotAdmin)
	assert.Empty(t, repo.unpublished)
}

func TestLerPublicacao_DevolveADoRepositorio(t *testing.T) {
	repo := &fakePublicationRepository{stored: publication.Publication{VaultID: "va", Slug: "docs"}}

	p, err := novoServico(repo).Publication(context.Background(), "va")

	require.NoError(t, err)
	assert.Equal(t, "docs", p.Slug)
}
