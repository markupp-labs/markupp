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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/vaults"
)

var instanteDoCofre = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

var (
	donaDoCofre = identity.Actor{UserID: "dona", TenantID: "t1", Role: "member"}
	colega      = identity.Actor{UserID: "colega", TenantID: "t1", Role: "member"}
	leitora     = identity.Actor{UserID: "leitora", TenantID: "t1", Role: "viewer"}
	deFora      = identity.Actor{UserID: "de-fora", TenantID: "t2", Role: "owner"}
)

func eventoDeCofre(ator identity.Actor, action, target string) auth.AuditEventRecord {
	return auth.AuditEventRecord{
		ID: "ev-" + action + "-" + target, TenantID: ator.TenantID, ActorID: ator.UserID, Action: action,
		Target: target, Result: "success", OccurredAt: instanteDoCofre,
	}
}

// semearTenantDeCofres grava a dona no cofre va, a colega no cofre vb e a leitora no cofre vl,
// todas no t1, e uma conta do t2. Devolve o contexto da dona.
func semearTenantDeCofres(t *testing.T, db *pgxpool.Pool) context.Context {
	t.Helper()
	storagetest.SeedMember(t, db, colega, "vb")
	storagetest.SeedMember(t, db, leitora, "vl")
	storagetest.SeedMember(t, db, deFora, "vf")
	return storagetest.SeedMember(t, db, donaDoCofre, "va")
}

func contarAuditoria(t *testing.T, db *pgxpool.Pool, action string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(context.Background(), "SELECT count(*) FROM audit_events WHERE action = $1", action).Scan(&n))
	return n
}

// emUTC tira o fuso local que o pgx põe no instante, para comparar só o momento.
func emUTC(m vaults.Member) vaults.Member {
	m.JoinedAt = m.JoinedAt.UTC()
	return m
}

func idsDosCofres(t *testing.T, repo *storage.PostgresVaultsRepository, ctx context.Context) []string {
	t.Helper()
	lista, err := repo.List(ctx)
	require.NoError(t, err)
	ids := make([]string, 0, len(lista))
	for _, v := range lista {
		ids = append(ids, v.ID)
	}
	return ids
}

func TestVaultsRepo_CreateTx_QuemCriaViraMembroEAuditoriaFica(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)
	repo := storage.NewPostgresVaultsRepository(db)

	novo := vaults.Vault{ID: "vn", Name: "Pesquisa", CreatedAt: instanteDoCofre}
	require.NoError(t, repo.CreateTx(ctx, novo, eventoDeCofre(donaDoCofre, "vaults.create", "vn")))

	assert.ElementsMatch(t, []string{"va", "vn"}, idsDosCofres(t, repo, ctx))
	assert.Equal(t, 1, contarAuditoria(t, db, "vaults.create"))
}

func TestVaultsRepo_List_SoCofresDaContaNoTenantDaSessao(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)

	assert.Equal(t, []string{"va"}, idsDosCofres(t, storage.NewPostgresVaultsRepository(db), ctx))
}

func TestVaultsRepo_RenameTx_TrocaONome(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)
	repo := storage.NewPostgresVaultsRepository(db)

	v, err := repo.RenameTx(ctx, "va", "Time", eventoDeCofre(donaDoCofre, "vaults.rename", "va"))

	require.NoError(t, err)
	assert.Equal(t, "Time", v.Name)
	assert.Equal(t, 1, contarAuditoria(t, db, "vaults.rename"))
}

func TestVaultsRepo_RenameTx_CofreAlheio_NaoEncontra(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)

	_, err := storage.NewPostgresVaultsRepository(db).RenameTx(ctx, "vb", "Meu", eventoDeCofre(donaDoCofre, "vaults.rename", "vb"))

	assert.ErrorIs(t, err, vaults.ErrVaultNotFound)
}

func TestVaultsRepo_DeleteTx_LevaAsNotas(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)
	storagetest.SaveNote(ctx, t, storage.NewPostgresNotesRepository(db, "simple"), notaNoCofre("n1", "a.md", "va"))

	require.NoError(t, storage.NewPostgresVaultsRepository(db).DeleteTx(ctx, "va", eventoDeCofre(donaDoCofre, "vaults.delete", "va")))

	var notas int
	require.NoError(t, db.QueryRow(context.Background(), "SELECT count(*) FROM notes WHERE id = 'n1'").Scan(&notas))
	assert.Zero(t, notas)
	assert.Equal(t, 1, contarAuditoria(t, db, "vaults.delete"))
}

func TestVaultsRepo_DeleteTx_CofreAlheio_NaoEncontra(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)

	err := storage.NewPostgresVaultsRepository(db).DeleteTx(ctx, "vb", eventoDeCofre(donaDoCofre, "vaults.delete", "vb"))

	assert.ErrorIs(t, err, vaults.ErrVaultNotFound)
}

func TestVaultsRepo_AddMemberTx_ColegaPassaAVerOCofre(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)
	repo := storage.NewPostgresVaultsRepository(db)

	m, err := repo.AddMemberTx(ctx, "va", "colega", instanteDoCofre, eventoDeCofre(donaDoCofre, "vaults.members.add", "colega"))

	require.NoError(t, err)
	assert.Equal(t, vaults.Member{UserID: "colega", Email: "colega@markupp.dev", Role: "member", JoinedAt: instanteDoCofre}, emUTC(m))
	assert.ElementsMatch(t, []string{"va", "vb"}, idsDosCofres(t, repo, identity.WithActor(context.Background(), colega)))
	assert.Equal(t, 1, contarAuditoria(t, db, "vaults.members.add"))
}

func TestVaultsRepo_AddMemberTx_Recusas(t *testing.T) {
	casos := []struct {
		nome, cofre, conta string
		esperado           error
	}{
		{"conta de outro tenant", "va", "de-fora", vaults.ErrNotTenantMember},
		{"conta inexistente", "va", "fantasma", vaults.ErrNotTenantMember},
		{"ja e membro", "va", "dona", vaults.ErrAlreadyVaultMember},
		{"cofre alheio", "vb", "leitora", vaults.ErrVaultNotFound},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db := setupTestDB(t)
			ctx := semearTenantDeCofres(t, db)

			_, err := storage.NewPostgresVaultsRepository(db).AddMemberTx(ctx, c.cofre, c.conta, instanteDoCofre, eventoDeCofre(donaDoCofre, "vaults.members.add", c.conta))

			assert.ErrorIs(t, err, c.esperado)
		})
	}
}

func TestVaultsRepo_ListMembers_ComEmailEPapelNoTenant(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)
	repo := storage.NewPostgresVaultsRepository(db)
	_, err := repo.AddMemberTx(ctx, "va", "leitora", instanteDoCofre, eventoDeCofre(donaDoCofre, "vaults.members.add", "leitora"))
	require.NoError(t, err)

	membros, err := repo.ListMembers(ctx, "va")

	require.NoError(t, err)
	require.Len(t, membros, 2)
	assert.Equal(t, "dona@markupp.dev", membros[0].Email)
	assert.Equal(t, vaults.Member{UserID: "leitora", Email: "leitora@markupp.dev", Role: "viewer", JoinedAt: instanteDoCofre}, emUTC(membros[1]))
}

func TestVaultsRepo_ListMembers_CofreAlheio_NaoEncontra(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)

	_, err := storage.NewPostgresVaultsRepository(db).ListMembers(ctx, "vb")

	assert.ErrorIs(t, err, vaults.ErrVaultNotFound)
}

func TestVaultsRepo_RemoveMemberTx_TiraAColega(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)
	repo := storage.NewPostgresVaultsRepository(db)
	_, err := repo.AddMemberTx(ctx, "va", "colega", instanteDoCofre, eventoDeCofre(donaDoCofre, "vaults.members.add", "colega"))
	require.NoError(t, err)

	require.NoError(t, repo.RemoveMemberTx(ctx, "va", "colega", eventoDeCofre(donaDoCofre, "vaults.members.remove", "colega")))

	assert.Equal(t, []string{"vb"}, idsDosCofres(t, repo, identity.WithActor(context.Background(), colega)))
	assert.Equal(t, 1, contarAuditoria(t, db, "vaults.members.remove"))
}

func TestVaultsRepo_RemoveMemberTx_LeitoraSaiSozinha(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)
	repo := storage.NewPostgresVaultsRepository(db)
	_, err := repo.AddMemberTx(ctx, "va", "leitora", instanteDoCofre, eventoDeCofre(donaDoCofre, "vaults.members.add", "leitora"))
	require.NoError(t, err)
	ctxLeitora := identity.WithActor(context.Background(), leitora)

	require.NoError(t, repo.RemoveMemberTx(ctxLeitora, "va", "leitora", eventoDeCofre(leitora, "vaults.members.remove", "leitora")))

	assert.Equal(t, []string{"vl"}, idsDosCofres(t, repo, ctxLeitora))
}

func TestVaultsRepo_RemoveMemberTx_QuemNaoEMembro_NaoEncontra(t *testing.T) {
	db := setupTestDB(t)
	ctx := semearTenantDeCofres(t, db)

	err := storage.NewPostgresVaultsRepository(db).RemoveMemberTx(ctx, "va", "colega", eventoDeCofre(donaDoCofre, "vaults.members.remove", "colega"))

	assert.ErrorIs(t, err, vaults.ErrMemberNotFound)
}

func TestRLS_LeitoraNaoGerenciaCofre(t *testing.T) {
	db := setupTestDB(t)
	semearTenantDeCofres(t, db)
	_, err := db.Exec(context.Background(), "INSERT INTO vault_members (vault_id, user_id, created_at) VALUES ('va', 'leitora', now())")
	require.NoError(t, err)

	escritas := map[string]string{
		"renomear":     "WITH u AS (UPDATE vaults SET name = 'x' WHERE id = 'va' RETURNING 1) SELECT count(*) FROM u",
		"apagar":       "WITH u AS (DELETE FROM vaults WHERE id = 'va' RETURNING 1) SELECT count(*) FROM u",
		"tirar a dona": "WITH u AS (DELETE FROM vault_members WHERE vault_id = 'va' AND user_id = 'dona' RETURNING 1) SELECT count(*) FROM u",
	}
	for nome, sql := range escritas {
		n, err := comoPapelDaAplicacao(t, db, leitora, sql)
		require.NoError(t, err, nome)
		assert.Zero(t, n, nome)
	}
	_, err = comoPapelDaAplicacao(t, db, leitora, "SELECT count(*) FROM markupp_create_vault('vx', 'X', now())")
	assert.Error(t, err, "leitora nao cria cofre")
	_, err = comoPapelDaAplicacao(t, db, leitora, "WITH u AS (INSERT INTO vault_members VALUES ('va', 'colega', now()) RETURNING 1) SELECT count(*) FROM u")
	assert.Error(t, err, "leitora nao adiciona membro")
}

func TestRLS_MembroNaoAdicionaContaDeForaDoTenant(t *testing.T) {
	db := setupTestDB(t)
	semearTenantDeCofres(t, db)

	_, err := comoPapelDaAplicacao(t, db, donaDoCofre, "WITH u AS (INSERT INTO vault_members VALUES ('va', 'de-fora', now()) RETURNING 1) SELECT count(*) FROM u")

	assert.Error(t, err)
}
