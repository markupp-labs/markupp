package storage_test

import (
	"context"
	"testing"

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
