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

func mudancaPor(ator, userID, papel string) auth.MembershipChange {
	c := mudanca(userID, papel)
	c.Event.ActorID = ator
	return c
}

func semRegra(auth.MembershipState) error { return nil }

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
	storagetest.SeedMember(t, db, identity.Actor{UserID: "rebaixada", TenantID: "t1", Role: "member"}, "vc")
	ctxVelho := identity.WithActor(context.Background(), identity.Actor{UserID: "rebaixada", TenantID: "t1", Role: "owner"})
	var estado auth.MembershipState
	_ = ctx

	err := storage.NewPostgresAdminRepository(db).ChangeMembershipTx(ctxVelho, mudancaPor("rebaixada", "socia", "admin"), func(e auth.MembershipState) error {
		estado = e
		return auth.ErrLastOwner
	})

	require.ErrorIs(t, err, auth.ErrLastOwner)
	assert.Equal(t, auth.MembershipState{Current: "owner", ActorRole: "member", Owners: 2}, estado,
		"papel do ator vem do banco, nao da requisicao")
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

func TestAdminRepo_RemoverMembro_ApagaCofreSoDeleSaiDosCompartilhadosECancelaConvites(t *testing.T) {
	db := setupTestDB(t)
	dona := identity.Actor{UserID: "dona", TenantID: "t1", Role: "owner"}
	ctxDona := storagetest.SeedMember(t, db, dona, "compartilhado")
	emitirConvite(ctxDona, t, db, conviteDe(dona, "c-antigo", "u2@markupp.dev"), "tok-antigo")
	ctxU2 := storagetest.SeedMember(t, db, identity.Actor{UserID: "u2", TenantID: "t1", Role: "member"}, "so-da-u2")
	_, err := db.Exec(context.Background(), "INSERT INTO vault_members (vault_id, user_id, created_at) VALUES ('compartilhado', 'u2', now())")
	require.NoError(t, err)
	require.NoError(t, storage.NewPostgresNotesRepository(db).Save(ctxU2, notaNoCofre("n-u2", "dela.md", "so-da-u2")))

	require.NoError(t, storage.NewPostgresAdminRepository(db).ChangeMembershipTx(ctxDona, mudanca("u2", ""), semRegra))

	var cofres, notas, pertencimentos int
	require.NoError(t, db.QueryRow(context.Background(), "SELECT count(*) FROM vaults WHERE id = 'so-da-u2'").Scan(&cofres))
	require.NoError(t, db.QueryRow(context.Background(), "SELECT count(*) FROM notes WHERE id = 'n-u2'").Scan(&notas))
	require.NoError(t, db.QueryRow(context.Background(), "SELECT count(*) FROM vault_members WHERE user_id = 'u2'").Scan(&pertencimentos))
	assert.Zero(t, cofres, "cofre em que so ela estava e apagado")
	assert.Zero(t, notas, "as notas vao junto com o cofre")
	assert.Zero(t, pertencimentos, "ela sai dos cofres compartilhados do tenant")
	_, err = storage.NewPostgresAuthRepository(db).AcceptInviteByIDTx(context.Background(), auth.AcceptInviteByIDParams{
		InviteID: "c-antigo", UserID: "u2", VaultID: "v-novo", AuditEventID: "ev-volta", Now: instanteDoConvite.Add(time.Minute),
	})
	assert.ErrorIs(t, err, auth.ErrInviteInvalid, "convite pendente nao devolve quem foi removido")
}
