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

var donaDoT1 = identity.Actor{UserID: "dona", TenantID: "t1", Role: "owner"}

func eventoDeApagarTenant(ator identity.Actor) auth.AuditEventRecord {
	return auth.AuditEventRecord{
		ID: "ev-apaga-" + ator.TenantID, TenantID: ator.TenantID, ActorID: ator.UserID, Action: "tenants.delete",
		Target: ator.TenantID, Result: "success", OccurredAt: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
	}
}

// semearDoisTenants enche t1 com cofres, notas, índice, convite com email e um membro, e
// t2 com uma nota da outra dona e um vínculo da dona de t1.
func semearDoisTenants(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	notas := storage.NewPostgresNotesRepository(db)
	ctxDona := storagetest.SeedMember(t, db, donaDoT1, "va")
	ctxU2 := storagetest.SeedMember(t, db, identity.Actor{UserID: "u2", TenantID: "t1", Role: "member"}, "vb")
	require.NoError(t, notas.Save(ctxDona, notaNoCofre("n-dona", "dela.md", "va")))
	require.NoError(t, notas.Save(ctxU2, notaNoCofre("n-u2", "dele.md", "vb")))
	inv := conviteDe(donaDoT1, "c1", "nova@markupp.dev")
	require.NoError(t, storage.NewPostgresAdminRepository(db).CreateInviteTx(ctxDona, inv, auth.HashOpaqueToken("tok"), eventoDoConvite(inv), true))
	_, err := db.Exec(context.Background(), "INSERT INTO note_search_index (note_id, document) VALUES ('n-dona', to_tsvector('dela'))")
	require.NoError(t, err)
	ctxOutra := storagetest.SeedMember(t, db, identity.Actor{UserID: "outra", TenantID: "t2", Role: "owner"}, "vc")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "dona", TenantID: "t2", Role: "member"}, "vd")
	require.NoError(t, notas.Save(ctxOutra, notaNoCofre("n-outra", "outra.md", "vc")))
}

func TestAdminRepo_DeleteTenantTx_ApagaTudoDoTenantEPreservaOOutroEAsContas(t *testing.T) {
	db := setupTestDB(t)
	semearDoisTenants(t, db)

	err := storage.NewPostgresAdminRepository(db).DeleteTenantTx(identity.WithActor(context.Background(), donaDoT1), eventoDeApagarTenant(donaDoT1))

	require.NoError(t, err)
	doT1 := map[string]string{
		"linha":        "SELECT count(*) FROM tenants WHERE id = 't1'",
		"vinculos":     "SELECT count(*) FROM tenant_memberships WHERE tenant_id = 't1'",
		"cofres":       "SELECT count(*) FROM vaults WHERE tenant_id = 't1'",
		"membros":      "SELECT count(*) FROM vault_members WHERE vault_id IN ('va', 'vb')",
		"notas":        "SELECT count(*) FROM notes WHERE tenant_id = 't1'",
		"fila":         "SELECT count(*) FROM note_index_queue WHERE note_id IN ('n-dona', 'n-u2')",
		"indice":       "SELECT count(*) FROM note_search_index WHERE note_id = 'n-dona'",
		"convites":     "SELECT count(*) FROM user_invites WHERE tenant_id = 't1'",
		"notificacoes": "SELECT count(*) FROM notifications WHERE tenant_id = 't1'",
	}
	for oque, consulta := range doT1 {
		assert.Zero(t, countRows(t, db, consulta), "%s do tenant apagado", oque)
	}
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM notes WHERE id = 'n-outra'"), "o outro tenant fica intacto")
	assert.Equal(t, 2, countRows(t, db, "SELECT count(*) FROM tenant_memberships WHERE tenant_id = 't2'"), "a dona segue no outro tenant")
	assert.Equal(t, 3, countRows(t, db, "SELECT count(*) FROM users WHERE id IN ('dona', 'u2', 'outra')"), "as contas continuam")
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM audit_events WHERE action = 'tenants.delete' AND tenant_id = 't1' AND actor_id = 'dona'"))
}

func TestRLS_SoOwnerApagaOTenant(t *testing.T) {
	for _, papel := range []string{"admin", "member", "viewer"} {
		db := setupTestDB(t)
		ator := identity.Actor{UserID: "u-" + papel, TenantID: "t1", Role: papel}
		storagetest.SeedMember(t, db, ator, "va")

		err := storage.NewPostgresAdminRepository(db).DeleteTenantTx(identity.WithActor(context.Background(), ator), eventoDeApagarTenant(ator))

		assert.ErrorIs(t, err, auth.ErrMembershipNotFound, papel)
		assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM tenants WHERE id = 't1'"), "%s nao apaga o tenant", papel)
	}
}
