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

func mudanca(userID, papel string) auth.MembershipChange {
	return auth.MembershipChange{UserID: userID, NewRole: papel, Event: auth.AuditEventRecord{
		ID: "ev-" + userID + papel, TenantID: "t1", ActorID: "dona", Action: "members.role", Target: userID, Result: "success",
		OccurredAt: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
	}}
}

func semRegra(current string, owners int) error { return nil }

func TestAdminRepo_ChangeMembershipTx_TrocaPapelERemove(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "dona", TenantID: "t1", Role: "owner"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u2", TenantID: "t1", Role: "member"}, "vb")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u3", TenantID: "t1", Role: "member"}, "vc")
	repo := storage.NewPostgresAdminRepository(db)

	require.NoError(t, repo.ChangeMembershipTx(ctx, mudanca("u2", "viewer"), semRegra))
	require.NoError(t, repo.ChangeMembershipTx(ctx, mudanca("u3", ""), semRegra))

	m, err := storage.NewPostgresAuthRepository(db).GetMembership(context.Background(), "u2", "t1")
	require.NoError(t, err)
	assert.Equal(t, "viewer", m.Role)
	_, err = storage.NewPostgresAuthRepository(db).GetMembership(context.Background(), "u3", "t1")
	assert.ErrorIs(t, err, auth.ErrMembershipNotFound)
}

func TestAdminRepo_ChangeMembershipTx_EntregaPapelAtualEOwnersParaARegra(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "dona", TenantID: "t1", Role: "owner"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "socia", TenantID: "t1", Role: "owner"}, "vb")
	var atual string
	var owners int

	err := storage.NewPostgresAdminRepository(db).ChangeMembershipTx(ctx, mudanca("socia", "admin"), func(c string, o int) error {
		atual, owners = c, o
		return auth.ErrLastOwner
	})

	require.ErrorIs(t, err, auth.ErrLastOwner)
	assert.Equal(t, "owner", atual)
	assert.Equal(t, 2, owners)
	m, err := storage.NewPostgresAuthRepository(db).GetMembership(context.Background(), "socia", "t1")
	require.NoError(t, err)
	assert.Equal(t, "owner", m.Role, "regra recusada nao muda nada")
}

func TestAdminRepo_ChangeMembershipTx_MembroInexistente_Recusa(t *testing.T) {
	db := setupTestDB(t)
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "dona", TenantID: "t1", Role: "owner"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "de-fora", TenantID: "t2", Role: "owner"}, "vb")

	err := storage.NewPostgresAdminRepository(db).ChangeMembershipTx(ctx, mudanca("de-fora", "member"), semRegra)

	assert.ErrorIs(t, err, auth.ErrMembershipNotFound)
}

func TestRLS_MembroNaoAlteraVinculos(t *testing.T) {
	db := setupTestDB(t)
	membro := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, membro, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "dona", TenantID: "t1", Role: "owner"}, "vb")

	n, err := comoPapelDaAplicacao(t, db, membro, "WITH u AS (UPDATE tenant_memberships SET role = 'owner' RETURNING 1) SELECT count(*) FROM u")

	require.NoError(t, err)
	assert.Zero(t, n, "membro nao promove ninguem, nem a si mesmo")
}
