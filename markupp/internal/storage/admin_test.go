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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

var instanteDoConvite = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func conviteDe(admin identity.Actor, id, email string) auth.Invite {
	return auth.Invite{
		ID: id, TenantID: admin.TenantID, Email: email, Role: "member", InvitedBy: admin.UserID,
		ExpiresAt: instanteDoConvite.Add(time.Hour), CreatedAt: instanteDoConvite,
	}
}

func eventoDoConvite(inv auth.Invite) auth.AuditEventRecord {
	return auth.AuditEventRecord{
		ID: "ev-" + inv.ID, TenantID: inv.TenantID, ActorID: inv.InvitedBy,
		Action: "users.invite", Target: inv.Email, Result: "success", OccurredAt: inv.CreatedAt,
	}
}

func emitirConvite(ctx context.Context, t *testing.T, db *pgxpool.Pool, inv auth.Invite, token string) {
	t.Helper()
	repo := storage.NewPostgresAdminRepository(db)
	require.NoError(t, repo.CreateInviteTx(ctx, inv, auth.HashOpaqueToken(token), eventoDoConvite(inv), false))
}

func TestAdminRepo_CreateInviteTx_ListaSoNoTenantEAudita(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAdminRepository(db)
	adminA := identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}
	ctxA := storagetest.SeedMember(t, db, adminA, "va")
	ctxB := storagetest.SeedMember(t, db, identity.Actor{UserID: "admin-b", TenantID: "t2", Role: "admin"}, "vb")

	emitirConvite(ctxA, t, db, conviteDe(adminA, "c1", "nova@markupp.dev"), "tok-1")

	doA, err := repo.ListInvites(ctxA)
	require.NoError(t, err)
	require.Len(t, doA, 1)
	assert.Equal(t, "nova@markupp.dev", doA[0].Email)
	assert.Nil(t, doA[0].AcceptedAt)
	doB, err := repo.ListInvites(ctxB)
	require.NoError(t, err)
	assert.Empty(t, doB)
	var auditados int
	require.NoError(t, db.QueryRow(context.Background(), "SELECT count(*) FROM audit_events WHERE action = 'users.invite' AND tenant_id = 't1'").Scan(&auditados))
	assert.Equal(t, 1, auditados)
}

func TestAdminRepo_CreateInviteTx_EmailJaUsuarioDoTenant_RecusaComErrAlreadyMember(t *testing.T) {
	db := setupTestDB(t)
	admin := identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}
	ctx := storagetest.SeedMember(t, db, admin, "va")
	repo := storage.NewPostgresAdminRepository(db)
	inv := conviteDe(admin, "c1", "admin-a@markupp.dev")

	err := repo.CreateInviteTx(ctx, inv, auth.HashOpaqueToken("tok-1"), eventoDoConvite(inv), false)

	assert.ErrorIs(t, err, auth.ErrAlreadyMember)
}

func TestAdminRepo_ListMembers_SoDoTenantDoAtorComPapel(t *testing.T) {
	db := setupTestDB(t)
	ctxA := storagetest.SeedMember(t, db, identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "membro-a", TenantID: "t1", Role: "viewer"}, "va2")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "admin-b", TenantID: "t2", Role: "admin"}, "vb")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "membro-a", TenantID: "t2", Role: "owner"}, "vc")

	membros, err := storage.NewPostgresAdminRepository(db).ListMembers(ctxA)

	require.NoError(t, err)
	require.Len(t, membros, 2)
	assert.Equal(t, "admin-a@markupp.dev", membros[0].Email)
	assert.Equal(t, "admin", membros[0].Role)
	assert.Equal(t, "membro-a@markupp.dev", membros[1].Email)
	assert.Equal(t, "viewer", membros[1].Role, "papel do tenant t1, nao o de t2")
}

func parametrosDeAceite(token string) auth.AcceptInviteParams {
	// #nosec G101 -- hash ficticio para teste
	return auth.AcceptInviteParams{
		TokenHash: auth.HashOpaqueToken(token), UserID: "novo", IdentityID: "ident-novo",
		PasswordHash: "hash-da-senha", InitialVaultID: "cofre-novo", AuditEventID: "ev-aceite",
		Now: instanteDoConvite.Add(time.Minute),
	}
}

func TestAuthRepo_AcceptInviteTx_CriaContaNoTenantEConsomeOConvite(t *testing.T) {
	db := setupTestDB(t)
	admin := identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}
	emitirConvite(storagetest.SeedMember(t, db, admin, "va"), t, db, conviteDe(admin, "c1", "nova@markupp.dev"), "tok-1")
	repo := storage.NewPostgresAuthRepository(db)

	user, err := repo.AcceptInviteTx(context.Background(), parametrosDeAceite("tok-1"))

	require.NoError(t, err)
	assert.Equal(t, auth.User{ID: "novo", Email: "nova@markupp.dev",
		CreatedAt: instanteDoConvite.Add(time.Minute), UpdatedAt: instanteDoConvite.Add(time.Minute)}, user)
	vinculo, err := repo.GetMembership(context.Background(), "novo", "t1")
	require.NoError(t, err)
	assert.Equal(t, "member", vinculo.Role)
	ident, err := repo.GetUserIdentity(context.Background(), "local", "nova@markupp.dev")
	require.NoError(t, err)
	assert.Equal(t, "hash-da-senha", ident.PasswordHash)
	_, err = repo.AcceptInviteTx(context.Background(), parametrosDeAceite("tok-1"))
	assert.ErrorIs(t, err, auth.ErrInviteInvalid, "convite aceito nao pode ser reusado")
}

func TestAuthRepo_AcceptInviteTx_ConviteVencido_RecusaComErrInviteInvalid(t *testing.T) {
	db := setupTestDB(t)
	admin := identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}
	emitirConvite(storagetest.SeedMember(t, db, admin, "va"), t, db, conviteDe(admin, "c1", "nova@markupp.dev"), "tok-1")
	params := parametrosDeAceite("tok-1")
	params.Now = instanteDoConvite.Add(2 * time.Hour)

	_, err := storage.NewPostgresAuthRepository(db).AcceptInviteTx(context.Background(), params)

	assert.ErrorIs(t, err, auth.ErrInviteInvalid)
}

func TestAdminRepo_CreateInviteTx_AtorMembro_BarradoPeloBanco(t *testing.T) {
	db := setupTestDB(t)
	membro := identity.Actor{UserID: "membro-a", TenantID: "t1", Role: "member"}
	ctx := storagetest.SeedMember(t, db, membro, "va")
	inv := conviteDe(membro, "c1", "nova@markupp.dev")

	err := storage.NewPostgresAdminRepository(db).CreateInviteTx(ctx, inv, auth.HashOpaqueToken("tok"), eventoDoConvite(inv), false)

	assert.Error(t, err, "so admin emite convite, mesmo que a rota esqueca o requireAdmin")
}
