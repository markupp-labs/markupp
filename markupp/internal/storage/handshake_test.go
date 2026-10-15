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

// conviteParaContaExistente emite, no tenant t1, convite de viewer para a conta
// "convidada", que já existe por estar no tenant t2.
func conviteParaContaExistente(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	admin := identity.Actor{UserID: "admin-a", TenantID: "t1", Role: "admin"}
	ctx := storagetest.SeedMember(t, db, admin, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "convidada", TenantID: "t2", Role: "owner"}, "vb")
	inv := conviteDe(admin, "c1", "convidada@markupp.dev")
	inv.Role = "viewer"
	emitirConvite(ctx, t, db, inv, "tok-1")
}

func aceite(userID string) auth.AcceptInviteByIDParams {
	return auth.AcceptInviteByIDParams{InviteID: "c1", UserID: userID, VaultID: "cofre-" + userID, AuditEventID: "ev-" + userID, Now: instanteDoConvite.Add(time.Minute)}
}

func TestHandshake_ListPendingInvites_MostraOConviteDoEmailDaConta(t *testing.T) {
	db := setupTestDB(t)
	conviteParaContaExistente(t, db)

	pendentes, err := storage.NewPostgresAuthRepository(db).ListPendingInvites(context.Background(), "convidada", instanteDoConvite)

	require.NoError(t, err)
	require.Len(t, pendentes, 1)
	assert.Equal(t, auth.PendingInvite{ID: "c1", TenantID: "t1", TenantName: "t1", Role: "viewer", ExpiresAt: instanteDoConvite.Add(time.Hour)}, pendentes[0])
	deOutro, err := storage.NewPostgresAuthRepository(db).ListPendingInvites(context.Background(), "admin-a", instanteDoConvite)
	require.NoError(t, err)
	assert.Empty(t, deOutro)
}

func TestHandshake_AcceptInviteByIDTx_VinculaComOPapelDoConviteUmaVez(t *testing.T) {
	db := setupTestDB(t)
	conviteParaContaExistente(t, db)
	repo := storage.NewPostgresAuthRepository(db)

	m, err := repo.AcceptInviteByIDTx(context.Background(), aceite("convidada"))

	require.NoError(t, err)
	assert.Equal(t, auth.Membership{TenantID: "t1", TenantName: "t1", Role: "viewer"}, m)
	_, err = repo.AcceptInviteByIDTx(context.Background(), aceite("convidada"))
	assert.ErrorIs(t, err, auth.ErrInviteInvalid, "convite respondido nao volta")
}

func TestHandshake_AcceptInviteByIDTx_ConviteDeOutroEmail_Recusa(t *testing.T) {
	db := setupTestDB(t)
	conviteParaContaExistente(t, db)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "intrusa", TenantID: "t3", Role: "owner"}, "vc")

	_, err := storage.NewPostgresAuthRepository(db).AcceptInviteByIDTx(context.Background(), aceite("intrusa"))

	assert.ErrorIs(t, err, auth.ErrInviteInvalid)
}

func TestHandshake_DeclineInviteTx_ImpedeAceiteDepois(t *testing.T) {
	db := setupTestDB(t)
	conviteParaContaExistente(t, db)
	repo := storage.NewPostgresAuthRepository(db)

	require.NoError(t, repo.DeclineInviteTx(context.Background(), auth.DeclineInviteParams{
		InviteID: "c1", UserID: "convidada", AuditEventID: "ev-recusa", Now: instanteDoConvite.Add(time.Minute),
	}))

	_, err := repo.AcceptInviteByIDTx(context.Background(), aceite("convidada"))
	assert.ErrorIs(t, err, auth.ErrInviteInvalid)
	_, err = repo.AcceptInviteTx(context.Background(), parametrosDeAceite("tok-1"))
	assert.ErrorIs(t, err, auth.ErrInviteInvalid, "o token tambem morre com a recusa")
}

func TestHandshake_AcceptInviteByIDTx_JaMembro_RecusaComErrAlreadyMember(t *testing.T) {
	db := setupTestDB(t)
	conviteParaContaExistente(t, db)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "convidada", TenantID: "t1", Role: "member"}, "vd")

	_, err := storage.NewPostgresAuthRepository(db).AcceptInviteByIDTx(context.Background(), aceite("convidada"))

	assert.ErrorIs(t, err, auth.ErrAlreadyMember)
}
