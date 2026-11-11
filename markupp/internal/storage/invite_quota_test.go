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

// cotaVista emite inv e devolve a cota que a transação mostrou ao check.
func cotaVista(ctx context.Context, t *testing.T, repo *storage.PostgresAdminRepository, inv auth.Invite) auth.InviteQuota {
	t.Helper()
	var vista auth.InviteQuota
	err := repo.CreateInviteTx(ctx, inv, auth.HashOpaqueToken("tok-"+inv.ID), eventoDoConvite(inv), false, func(q auth.InviteQuota) error {
		vista = q
		return nil
	})
	require.NoError(t, err)
	return vista
}

func TestAdminRepo_CreateInviteTx_CotaContaConvitesRevogadosEDeOutrosTenants(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAdminRepository(db)
	adminT1 := identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}
	adminT2 := identity.Actor{UserID: "admin-a", TenantID: "t2", Role: "admin"}
	ctxT1 := storagetest.SeedMember(t, db, adminT1, "va")
	ctxT2 := storagetest.SeedMember(t, db, adminT2, "vb")
	emitirConvite(ctxT1, t, db, conviteDe(adminT1, "c1", "um@markupp.dev"), "tok-1")
	require.NoError(t, repo.RevokeInviteTx(ctxT1, "c1", auth.AuditEventRecord{
		ID: "ev-revoga", TenantID: "t1", ActorID: "admin-a", Action: "users.invite_revoke",
		Target: "c1", Result: "success", OccurredAt: instanteDoConvite,
	}))
	emitirConvite(ctxT2, t, db, conviteDe(adminT2, "c2", "dois@markupp.dev"), "tok-2")

	cota := cotaVista(ctxT1, t, repo, conviteDe(adminT1, "c3", "tres@markupp.dev"))

	assert.True(t, cota.InviterVerified)
	assert.Equal(t, 2, cota.ByInviter, "o revogado e o do outro tenant contam para a conta")
	assert.Equal(t, 1, cota.ByTenant, "o revogado conta para o tenant, o do outro tenant nao")
}

func TestAdminRepo_CreateInviteTx_CotaIgnoraConvitesForaDaJanela(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAdminRepository(db)
	admin := identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}
	ctx := storagetest.SeedMember(t, db, admin, "va")
	antigo := conviteDe(admin, "c1", "um@markupp.dev")
	antigo.CreatedAt = instanteDoConvite.Add(-auth.InviteWindow - time.Minute)
	emitirConvite(ctx, t, db, antigo, "tok-1")

	cota := cotaVista(ctx, t, repo, conviteDe(admin, "c2", "dois@markupp.dev"))

	assert.Zero(t, cota.ByInviter)
	assert.Zero(t, cota.ByTenant)
}

func TestAdminRepo_CreateInviteTx_CotaMostraEmailNaoVerificado(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAdminRepository(db)
	admin := identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}
	ctx := storagetest.SeedMember(t, db, admin, "va")
	_, err := db.Exec(context.Background(), "UPDATE users SET email_verified = FALSE WHERE id = 'admin-a'")
	require.NoError(t, err)

	cota := cotaVista(ctx, t, repo, conviteDe(admin, "c1", "um@markupp.dev"))

	assert.False(t, cota.InviterVerified)
}

func TestAdminRepo_CreateInviteTx_CheckRecusa_NaoGravaConviteNemAuditoria(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAdminRepository(db)
	admin := identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}
	ctx := storagetest.SeedMember(t, db, admin, "va")
	inv := conviteDe(admin, "c1", "um@markupp.dev")

	err := repo.CreateInviteTx(ctx, inv, auth.HashOpaqueToken("tok"), eventoDoConvite(inv), true, func(auth.InviteQuota) error {
		return auth.ErrInviteLimit
	})

	assert.ErrorIs(t, err, auth.ErrInviteLimit)
	var convites, eventos int
	require.NoError(t, db.QueryRow(context.Background(), "SELECT count(*) FROM user_invites").Scan(&convites))
	require.NoError(t, db.QueryRow(context.Background(), "SELECT count(*) FROM audit_events WHERE action = 'users.invite'").Scan(&eventos))
	assert.Zero(t, convites)
	assert.Zero(t, eventos)
}
