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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

func TestAuthRepo_GetMembership_DevolvePapelNoTenant(t *testing.T) {
	db := setupTestDB(t)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u1", TenantID: "t1", Role: "viewer"}, "va")

	m, err := storage.NewPostgresAuthRepository(db).GetMembership(context.Background(), "u1", "t1")

	require.NoError(t, err)
	assert.Equal(t, auth.Membership{TenantID: "t1", TenantName: "t1", Role: "viewer"}, m)
}

func TestAuthRepo_GetMembership_SemVinculo_RecusaComErrMembershipNotFound(t *testing.T) {
	db := setupTestDB(t)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u1", TenantID: "t1", Role: "member"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u2", TenantID: "t2", Role: "owner"}, "vb")

	_, err := storage.NewPostgresAuthRepository(db).GetMembership(context.Background(), "u1", "t2")

	assert.ErrorIs(t, err, auth.ErrMembershipNotFound)
}

func TestAuthRepo_ListMemberships_SoOsDoUsuario(t *testing.T) {
	db := setupTestDB(t)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u1", TenantID: "t1", Role: "member"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u1", TenantID: "t2", Role: "owner"}, "vb")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u2", TenantID: "t3", Role: "owner"}, "vc")

	lista, err := storage.NewPostgresAuthRepository(db).ListMemberships(context.Background(), "u1")

	require.NoError(t, err)
	assert.Equal(t, []auth.Membership{
		{TenantID: "t1", TenantName: "t1", Role: "member"},
		{TenantID: "t2", TenantName: "t2", Role: "owner"},
	}, lista)
}

func TestAuthRepo_CreateTenantTx_CriadorViraOwnerComCofreEAuditoria(t *testing.T) {
	db := setupTestDB(t)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u1", TenantID: "t1", Role: "member"}, "va")
	repo := storage.NewPostgresAuthRepository(db)
	agora := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

	err := repo.CreateTenantTx(context.Background(), auth.CreateTenantParams{
		TenantID: "novo", Name: "Acme Ltda", UserID: "u1", VaultID: "cofre-novo", AuditEventID: "ev-novo", Now: agora,
	})

	require.NoError(t, err)
	m, err := repo.GetMembership(context.Background(), "u1", "novo")
	require.NoError(t, err)
	assert.Equal(t, auth.Membership{TenantID: "novo", TenantName: "Acme Ltda", Role: "owner"}, m)
	ctx := identity.WithActor(context.Background(), identity.Actor{UserID: "u1", TenantID: "novo", Role: "owner"})
	padrao, err := storage.NewPostgresNotesRepository(db, "simple").DefaultVaultID(ctx)
	require.NoError(t, err)
	assert.Equal(t, "cofre-novo", padrao)
	var auditados int
	require.NoError(t, db.QueryRow(context.Background(), "SELECT count(*) FROM audit_events WHERE action = 'tenants.create' AND tenant_id = 'novo'").Scan(&auditados))
	assert.Equal(t, 1, auditados)
}

func salvarNotaEm(ctx context.Context, t *testing.T, db *pgxpool.Pool, id, vaultID string, updatedAt time.Time) {
	t.Helper()
	nota := notes.Note{ID: id, VaultID: vaultID, Path: id + ".md", Content: id, CreatedAt: updatedAt, UpdatedAt: updatedAt}
	_, err := storage.NewPostgresNotesRepository(db, "simple").Save(ctx, nota)
	require.NoError(t, err)
}

func TestAuthRepo_ListTenantSummaries_ContaSoAsNotasQueAContaLe(t *testing.T) {
	db := setupTestDB(t)
	cedo := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	tarde := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	ctxU1 := storagetest.SeedMember(t, db, identity.Actor{UserID: "u1", TenantID: "t1", Role: "member"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u1", TenantID: "t2", Role: "viewer"}, "vb")
	ctxU2 := storagetest.SeedMember(t, db, identity.Actor{UserID: "u2", TenantID: "t1", Role: "member"}, "vx")
	salvarNotaEm(ctxU1, t, db, "n1", "va", cedo)
	salvarNotaEm(ctxU1, t, db, "n2", "va", tarde)
	salvarNotaEm(ctxU2, t, db, "n3", "vx", tarde.Add(time.Hour))

	lista, err := storage.NewPostgresAuthRepository(db).ListTenantSummaries(context.Background(), "u1")

	require.NoError(t, err)
	require.Len(t, lista, 2)
	assert.Equal(t, auth.Membership{TenantID: "t1", TenantName: "t1", Role: "member"}, lista[0].Membership)
	assert.Equal(t, 2, lista[0].NotesCount, "a nota do cofre de u2 fica fora")
	require.NotNil(t, lista[0].LastNoteAt)
	assert.True(t, tarde.Equal(*lista[0].LastNoteAt), "esperado %s, veio %s", tarde, lista[0].LastNoteAt)
	assert.Equal(t, auth.TenantSummary{Membership: auth.Membership{TenantID: "t2", TenantName: "t2", Role: "viewer"}}, lista[1])
}

func TestAuthRepo_ListTenantSummaries_BateComOQueORLSDeixaLer(t *testing.T) {
	db := setupTestDB(t)
	agora := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	ctxU1 := storagetest.SeedMember(t, db, identity.Actor{UserID: "u1", TenantID: "t1", Role: "viewer"}, "va")
	ctxU2 := storagetest.SeedMember(t, db, identity.Actor{UserID: "u2", TenantID: "t1", Role: "owner"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u2", TenantID: "t1", Role: "owner"}, "vy")
	salvarNotaEm(ctxU2, t, db, "compartilhada", "va", agora)
	salvarNotaEm(ctxU2, t, db, "so-de-u2", "vy", agora)

	lista, err := storage.NewPostgresAuthRepository(db).ListTenantSummaries(context.Background(), "u1")
	require.NoError(t, err)
	visiveis, err := storage.NewPostgresNotesRepository(db, "simple").ListNotes(ctxU1)
	require.NoError(t, err)

	require.Len(t, lista, 1)
	assert.Equal(t, len(visiveis), lista[0].NotesCount)
	assert.Equal(t, 1, lista[0].NotesCount)
}
