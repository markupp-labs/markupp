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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

var (
	adminDoCofre   = identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}
	momentoPublico = time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
)

func publicacaoDe(vaultID, slug, title string) publication.Publication {
	return publication.Publication{VaultID: vaultID, Slug: slug, Title: title, PublishedBy: adminDoCofre.UserID, PublishedAt: momentoPublico}
}

func eventoDePublicacao(id, action, vaultID string) auth.AuditEventRecord {
	return auth.AuditEventRecord{ID: id, TenantID: "t1", ActorID: adminDoCofre.UserID, Action: action, Target: vaultID, Result: "success", OccurredAt: momentoPublico}
}

// emUTC tira o fuso local que o driver põe no instante lido do banco.
func emUTC(p publication.Publication) publication.Publication {
	p.PublishedAt = p.PublishedAt.UTC()
	return p
}

// cofreDoAdmin semeia adminDoCofre no cofre "va" e devolve o contexto dele.
func cofreDoAdmin(t *testing.T) (*pgxpool.Pool, context.Context, *storage.PostgresVaultPublications) {
	t.Helper()
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, adminDoCofre, "va")
	return db, ctx, storage.NewPostgresVaultPublications(db)
}

func TestPublicarCofre_AdminDoCofre_GravaEAudita(t *testing.T) {
	db, ctx, repo := cofreDoAdmin(t)

	p, err := repo.PublishTx(ctx, publicacaoDe("va", "docs", "Documentacao"), eventoDePublicacao("e1", publication.ActionPublish, "va"))

	require.NoError(t, err)
	assert.Equal(t, publicacaoDe("va", "docs", "Documentacao"), emUTC(p))
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM audit_events WHERE action = 'vaults.publish' AND target = 'va'"))
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM vault_publications WHERE tenant_id = 't1' AND slug = 'docs'"))
}

func TestPublicarCofre_SemTitulo_UsaONomeDoCofre(t *testing.T) {
	_, ctx, repo := cofreDoAdmin(t)

	p, err := repo.PublishTx(ctx, publicacaoDe("va", "docs", ""), eventoDePublicacao("e1", publication.ActionPublish, "va"))

	require.NoError(t, err)
	assert.Equal(t, "va", p.Title, "SeedMember nomeia o cofre com o id")
}

func TestPublicarCofre_DeNovo_TrocaSlugETituloEMantemAPrimeiraPublicacao(t *testing.T) {
	_, ctx, repo := cofreDoAdmin(t)
	_, err := repo.PublishTx(ctx, publicacaoDe("va", "docs", "A"), eventoDePublicacao("e1", publication.ActionPublish, "va"))
	require.NoError(t, err)
	depois := publicacaoDe("va", "manual", "B")
	depois.PublishedAt = momentoPublico.Add(time.Hour)

	p, err := repo.PublishTx(ctx, depois, eventoDePublicacao("e2", publication.ActionPublish, "va"))

	require.NoError(t, err)
	assert.Equal(t, publicacaoDe("va", "manual", "B"), emUTC(p))
}

func TestPublicarCofre_SlugDeOutroTenant_DevolveSlugTomado(t *testing.T) {
	db, ctx, repo := cofreDoAdmin(t)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "outro", TenantID: "t2", Role: "owner"}, "vb")
	publicaCofre(t, db, "t2", "vb", "docs")

	_, err := repo.PublishTx(ctx, publicacaoDe("va", "docs", "A"), eventoDePublicacao("e1", publication.ActionPublish, "va"))

	require.ErrorIs(t, err, publication.ErrSlugTaken)
	assert.Contains(t, err.Error(), `"docs"`)
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM audit_events"), "falha nao audita")
}

func TestPublicarCofre_MembroDoCofre_Recusado(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "membro", TenantID: "t1", Role: "member"}, "va")

	_, err := storage.NewPostgresVaultPublications(db).PublishTx(ctx, publicacaoDe("va", "docs", "A"), eventoDePublicacao("e1", publication.ActionPublish, "va"))

	require.ErrorIs(t, err, notes.ErrVaultForbidden)
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM vault_publications"))
}

func TestPublicarCofre_CofreDeQueNaoEMembro_Recusado(t *testing.T) {
	db, ctx, repo := cofreDoAdmin(t)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "colega", TenantID: "t1", Role: "admin"}, "vc")

	_, err := repo.PublishTx(ctx, publicacaoDe("vc", "docs", "A"), eventoDePublicacao("e1", publication.ActionPublish, "vc"))

	require.ErrorIs(t, err, notes.ErrVaultForbidden)
	assert.Contains(t, err.Error(), `"vc"`)
}

func TestDespublicarCofre_Publicado_ApagaEAudita(t *testing.T) {
	db, ctx, repo := cofreDoAdmin(t)
	publicaCofre(t, db, "t1", "va", "docs")

	err := repo.UnpublishTx(ctx, "va", eventoDePublicacao("e1", publication.ActionUnpublish, "va"))

	require.NoError(t, err)
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM vault_publications"))
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM audit_events WHERE action = 'vaults.unpublish'"))
}

func TestDespublicarCofre_NaoPublicado_DevolveNaoPublicado(t *testing.T) {
	_, ctx, repo := cofreDoAdmin(t)

	err := repo.UnpublishTx(ctx, "va", eventoDePublicacao("e1", publication.ActionUnpublish, "va"))

	require.ErrorIs(t, err, publication.ErrNotPublished)
}

func TestDespublicarCofre_Viewer_RecusadoEContinuaPublicado(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "leitor", TenantID: "t1", Role: "viewer"}, "va")
	publicaCofre(t, db, "t1", "va", "docs")

	err := storage.NewPostgresVaultPublications(db).UnpublishTx(ctx, "va", eventoDePublicacao("e1", publication.ActionUnpublish, "va"))

	require.ErrorIs(t, err, notes.ErrVaultForbidden)
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM vault_publications"))
}

// Regressão: owner que não é membro do cofre recebia "nao publicado" e o cofre
// continuava público.
func TestDespublicarCofre_OwnerForaDoCofre_RecusadoEContinuaPublicado(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "dono", TenantID: "t1", Role: "owner"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "colega", TenantID: "t1", Role: "admin"}, "vc")
	publicaCofre(t, db, "t1", "vc", "docs")

	err := storage.NewPostgresVaultPublications(db).UnpublishTx(ctx, "vc", eventoDePublicacao("e1", publication.ActionUnpublish, "vc"))

	require.ErrorIs(t, err, notes.ErrVaultForbidden)
	assert.Contains(t, err.Error(), `"vc"`)
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM vault_publications WHERE vault_id = 'vc'"))
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM audit_events"), "falha nao audita")
}

func TestLerPublicacao_ViewerDoCofre_Le(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "leitor", TenantID: "t1", Role: "viewer"}, "va")
	publicaCofre(t, db, "t1", "va", "docs")

	p, err := storage.NewPostgresVaultPublications(db).Publication(ctx, "va")

	require.NoError(t, err)
	assert.Equal(t, "docs", p.Slug)
	assert.Equal(t, "quem-publicou", p.PublishedBy)
}

func TestLerPublicacao_CofreAlheio_Recusado(t *testing.T) {
	db, ctx, repo := cofreDoAdmin(t)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "outro", TenantID: "t2", Role: "owner"}, "vb")
	publicaCofre(t, db, "t2", "vb", "docs")

	_, err := repo.Publication(ctx, "vb")

	require.ErrorIs(t, err, notes.ErrVaultForbidden)
}

func TestLerPublicacao_AdminForaDoCofre_Recusado(t *testing.T) {
	db, ctx, repo := cofreDoAdmin(t)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "colega", TenantID: "t1", Role: "admin"}, "vc")
	publicaCofre(t, db, "t1", "vc", "docs")

	_, err := repo.Publication(ctx, "vc")

	require.ErrorIs(t, err, notes.ErrVaultForbidden)
	assert.Contains(t, err.Error(), `"vc"`)
}

func TestLerPublicacao_NaoPublicado_DevolveNaoPublicado(t *testing.T) {
	_, ctx, repo := cofreDoAdmin(t)

	_, err := repo.Publication(ctx, "va")

	require.ErrorIs(t, err, publication.ErrNotPublished)
}
