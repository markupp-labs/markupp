package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

func notaNoCofre(id, path, vaultID string) notes.Note {
	agora := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	return notes.Note{ID: id, VaultID: vaultID, Path: path, Content: "segredo " + path, CreatedAt: agora, UpdatedAt: agora}
}

func assertNotaInvisivel(ctx context.Context, t *testing.T, repo *storage.PostgresNotesRepository, id string) {
	t.Helper()
	_, err := repo.GetNoteByID(ctx, id)
	assert.ErrorIs(t, err, notes.ErrNotFound)
	lista, err := repo.ListNotes(ctx)
	require.NoError(t, err)
	assert.Empty(t, lista)
	achadas, err := repo.SearchNotes(ctx, "segredo", 0, 10)
	require.NoError(t, err)
	assert.Empty(t, achadas)
	_, err = repo.Update(ctx, id, "invadida.md", "x", time.Now(), time.Time{}, true)
	assert.ErrorIs(t, err, notes.ErrNotFound)
	assert.ErrorIs(t, repo.Delete(ctx, id), notes.ErrNotFound)
}

func TestNotesRLS_OutroTenant_NaoEnxergaNemAlteraNota(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctxA := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	ctxB := storagetest.SeedMember(t, db, identity.Actor{UserID: "ub", TenantID: "t2", Role: "admin"}, "vb")
	require.NoError(t, repo.Save(ctxA, notaNoCofre("n1", "a.md", "va")))

	assertNotaInvisivel(ctxB, t, repo, "n1")

	_, err := repo.GetNoteByID(ctxA, "n1")
	assert.NoError(t, err, "a dona da nota continua enxergando depois das tentativas alheias")
}

func TestNotesRLS_MesmoTenantForaDoCofre_NaoEnxergaNota(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctxA := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	ctxB := storagetest.SeedMember(t, db, identity.Actor{UserID: "ub", TenantID: "t1", Role: "admin"}, "vb")
	require.NoError(t, repo.Save(ctxA, notaNoCofre("n1", "a.md", "va")))

	assertNotaInvisivel(ctxB, t, repo, "n1")
}

func TestNotesRLS_CofreSemAcesso_RecusaEscrita(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	ctxB := storagetest.SeedMember(t, db, identity.Actor{UserID: "ub", TenantID: "t1", Role: "member"}, "vb")

	assert.ErrorIs(t, repo.Save(ctxB, notaNoCofre("n1", "a.md", "va")), notes.ErrVaultForbidden)
	assert.ErrorIs(t, repo.Save(ctxB, notaNoCofre("n2", "b.md", "inexistente")), notes.ErrVaultForbidden)
}

func TestNotesRLS_DefaultVaultID_DevolveOCofreMaisAntigoDoAtor(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ator := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, ator, "primeiro")
	ctx := storagetest.SeedMember(t, db, ator, "segundo")

	vaultID, err := repo.DefaultVaultID(ctx)

	require.NoError(t, err)
	assert.Equal(t, "primeiro", vaultID)
}

func TestNotesRLS_DefaultVaultID_AtorSemCofre_RecusaComErrVaultForbidden(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := identity.WithActor(context.Background(), identity.Actor{UserID: "orfao", TenantID: "t1", Role: "member"})

	_, err := repo.DefaultVaultID(ctx)

	assert.ErrorIs(t, err, notes.ErrVaultForbidden)
}

func TestNotesRLS_MesmoCaminhoEmCofresDiferentes_Permitido(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ator := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, ator, "va")
	ctx := storagetest.SeedMember(t, db, ator, "vb")

	require.NoError(t, repo.Save(ctx, notaNoCofre("n1", "igual.md", "va")))
	require.NoError(t, repo.Save(ctx, notaNoCofre("n2", "igual.md", "vb")))
	assert.ErrorIs(t, repo.Save(ctx, notaNoCofre("n3", "igual.md", "va")), notes.ErrDuplicatePath)
}

func TestNotesRepo_SemAtorNoContexto_RecusaComErrNoActor(t *testing.T) {
	repo := storage.NewPostgresNotesRepository(setupTestDB(t))

	_, err := repo.ListNotes(context.Background())

	assert.ErrorIs(t, err, storage.ErrNoActor)
}

// comoPapelDaAplicacao roda sql no papel markupp_app com o tenant, o usuário e o papel
// do ator, sem passar pelo repositório: prova a política do banco, não a disciplina
// da query.
func comoPapelDaAplicacao(t *testing.T, db *pgxpool.Pool, ator identity.Actor, sql string) (int64, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "SELECT set_config('role', 'markupp_app', true), set_config('markupp.tenant_id', $1, true), set_config('markupp.user_id', $2, true), set_config('markupp.role', $3, true)",
		ator.TenantID, ator.UserID, ator.Role)
	require.NoError(t, err)
	var n int64
	err = tx.QueryRow(ctx, sql).Scan(&n)
	return n, err
}

func TestRLS_PapelDaAplicacaoSemFiltro_SoEnxergaOProprioTenant(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctxA := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "ub", TenantID: "t2", Role: "member"}, "vb")
	require.NoError(t, repo.Save(ctxA, notaNoCofre("n1", "a.md", "va")))

	consultasDoCofreAlheio := []string{
		"SELECT count(*) FROM notes WHERE vault_id = 'va'",
		"SELECT count(*) FROM vaults WHERE id = 'va'",
		"SELECT count(*) FROM vault_members WHERE vault_id = 'va'",
	}
	for _, sql := range consultasDoCofreAlheio {
		n, err := comoPapelDaAplicacao(t, db, identity.Actor{UserID: "ub", TenantID: "t2", Role: "admin"}, sql)
		require.NoError(t, err, sql)
		assert.Zero(t, n, "tenant t2 enxergou linha do cofre va: %s", sql)
	}
	n, err := comoPapelDaAplicacao(t, db, identity.Actor{UserID: "ub", TenantID: "t2", Role: "admin"}, "SELECT count(*) FROM users")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "tenant t2 deve ver so o proprio usuario")
}

func TestRLS_PapelDaAplicacao_NaoAlteraAuditoria(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.Exec(context.Background(),
		"INSERT INTO audit_events (id, tenant_id, action, target, result, occurred_at) VALUES ('e1', 't1', 'auth.login', 'x', 'success', now())")
	require.NoError(t, err)

	_, err = comoPapelDaAplicacao(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "admin"}, "WITH d AS (DELETE FROM audit_events RETURNING 1) SELECT count(*) FROM d")

	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "42501", pgErr.Code, "esperada permissao negada ao papel da aplicacao, recebido %v", err)
}

func TestRLS_PapelDaAplicacaoComoMembro_VeOsColegasMasNaoOsConvites(t *testing.T) {
	db := setupTestDB(t)
	membro := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, membro, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "colega", TenantID: "t1", Role: "admin"}, "vc")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "de-fora", TenantID: "t2", Role: "owner"}, "vd")

	colegas, err := comoPapelDaAplicacao(t, db, membro, "SELECT count(*) FROM users")
	require.NoError(t, err)
	assert.Equal(t, int64(2), colegas, "membro ve quem e do tenant, e so isso")
	convites, err := comoPapelDaAplicacao(t, db, membro, "SELECT count(*) FROM user_invites")
	require.NoError(t, err)
	assert.Zero(t, convites)
}

func TestNotesRLS_Viewer_LeMasNaoEscreve(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctxDona := storagetest.SeedMember(t, db, identity.Actor{UserID: "dona", TenantID: "t1", Role: "owner"}, "va")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "leitora", TenantID: "t1", Role: "viewer"}, "va")
	ctxLeitora := identity.WithActor(context.Background(), identity.Actor{UserID: "leitora", TenantID: "t1", Role: "viewer"})
	require.NoError(t, repo.Save(ctxDona, notaNoCofre("n1", "a.md", "va")))

	_, err := repo.GetNoteByID(ctxLeitora, "n1")
	require.NoError(t, err, "viewer le as notas do cofre")
	assert.ErrorIs(t, repo.Save(ctxLeitora, notaNoCofre("n2", "b.md", "va")), notes.ErrVaultForbidden)
	_, err = repo.Update(ctxLeitora, "n1", "a.md", "alterada", time.Now(), time.Time{}, true)
	assert.ErrorIs(t, err, notes.ErrNotFound)
	assert.ErrorIs(t, repo.Delete(ctxLeitora, "n1"), notes.ErrNotFound)
	intacta, err := repo.GetNoteByID(ctxDona, "n1")
	require.NoError(t, err)
	assert.Equal(t, "segredo a.md", intacta.Content)
}

func TestNotesRLS_DefaultVaultID_ContaEmDoisTenants_UsaOCofreDoTenantDaSessao(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	storagetest.SeedMember(t, db, identity.Actor{UserID: "bia", TenantID: "t1", Role: "member"}, "cofre-antigo-t1")
	ctxT2 := storagetest.SeedMember(t, db, identity.Actor{UserID: "bia", TenantID: "t2", Role: "owner"}, "cofre-t2")

	vaultID, err := repo.DefaultVaultID(ctxT2)

	require.NoError(t, err)
	assert.Equal(t, "cofre-t2", vaultID, "o cofre padrao e do tenant em que a requisicao age")
	n, err := comoPapelDaAplicacao(t, db, identity.Actor{UserID: "bia", TenantID: "t2", Role: "owner"},
		"SELECT count(*) FROM vault_members WHERE vault_id = 'cofre-antigo-t1'")
	require.NoError(t, err)
	assert.Zero(t, n, "pertencimento a cofre de outro tenant nao aparece na sessao")
}
