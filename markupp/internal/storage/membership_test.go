package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
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
