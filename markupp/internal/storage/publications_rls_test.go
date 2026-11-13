package storage_test

import (
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

var (
	donaDoTenant  = identity.Actor{UserID: "dona", TenantID: "t1", Role: "owner"}
	adminDeFora   = identity.Actor{UserID: "admin-fora", TenantID: "t1", Role: "admin"}
	membroDoCofre = identity.Actor{UserID: "membro", TenantID: "t1", Role: "member"}
	leitorDoCofre = identity.Actor{UserID: "leitor", TenantID: "t1", Role: "viewer"}
	donoDeOutro   = identity.Actor{UserID: "outro", TenantID: "t2", Role: "owner"}
)

// tenantComDoisCofres semeia, no tenant t1, o cofre "va" com a dona, o membro e o
// viewer e a nota "na"; o cofre "vb" só com o admin de fora, com a nota "nb"; e o
// tenant t2 com o cofre "vc".
func tenantComDoisCofres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctx := storagetest.SeedMember(t, db, donaDoTenant, "va")
	storagetest.SeedMember(t, db, membroDoCofre, "va")
	storagetest.SeedMember(t, db, leitorDoCofre, "va")
	ctxB := storagetest.SeedMember(t, db, adminDeFora, "vb")
	storagetest.SeedMember(t, db, donoDeOutro, "vc")
	storagetest.SaveNote(ctx, t, repo, notaEm("va", "na", "docs/na.md"))
	storagetest.SaveNote(ctxB, t, repo, notaEm("vb", "nb", "nb.md"))
	return db
}

// insercaoDePublicacao é o INSERT da aplicação, que deixa tenant_id para o DEFAULT.
func insercaoDePublicacao(ator identity.Actor, vaultID string, alvo alvoDePublicacao) string {
	return fmt.Sprintf(`WITH i AS (INSERT INTO publications (id, vault_id, kind, note_id, folder_path, published_by, published_at)
		VALUES ('nova', '%s', '%s', NULLIF('%s', ''), NULLIF('%s', ''), '%s', now()) RETURNING 1) SELECT count(*) FROM i`,
		vaultID, alvo.kind, alvo.noteID, alvo.folder, ator.UserID)
}

func TestRLS_Publicar_QuemPodeEditarANotaPublicaANota(t *testing.T) {
	db := tenantComDoisCofres(t)

	for _, ator := range []identity.Actor{donaDoTenant, membroDoCofre} {
		n, err := comoPapelDaAplicacao(t, db, ator, insercaoDePublicacao(ator, "va", alvoNota("na")))
		require.NoError(t, err, ator.Role)
		assert.Equal(t, int64(1), n, ator.Role)
	}
}

func TestRLS_Publicar_RecusaQuemNaoPodePublicar(t *testing.T) {
	db := tenantComDoisCofres(t)
	recusas := []struct {
		motivo string
		ator   identity.Actor
		cofre  string
		alvo   alvoDePublicacao
	}{
		{"viewer publicando nota", leitorDoCofre, "va", alvoNota("na")},
		{"membro publicando pasta", membroDoCofre, "va", alvoPasta("docs")},
		{"membro publicando cofre", membroDoCofre, "va", alvoCofre()},
		{"admin fora do cofre publicando o cofre", adminDeFora, "va", alvoCofre()},
		{"nota de outro cofre", donaDoTenant, "va", alvoNota("nb")},
		{"cofre de outro tenant", donoDeOutro, "va", alvoCofre()},
	}
	for _, r := range recusas {
		_, err := comoPapelDaAplicacao(t, db, r.ator, insercaoDePublicacao(r.ator, r.cofre, r.alvo))
		requirePermissaoNegada(t, err, r.motivo)
	}
}

func TestRLS_Publicar_RecusaPublicarEmNomeDeOutraConta(t *testing.T) {
	db := tenantComDoisCofres(t)

	_, err := comoPapelDaAplicacao(t, db, donaDoTenant, insercaoDePublicacao(membroDoCofre, "va", alvoCofre()))

	requirePermissaoNegada(t, err, "published_by de outra conta")
}

// duasPublicacoes publica a pasta docs de "va" e a nota "nb" de "vb".
func duasPublicacoes(t *testing.T) *pgxpool.Pool {
	t.Helper()
	db := tenantComDoisCofres(t)
	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))
	publica(t, db, "p-nb", "t1", "vb", alvoNota("nb"))
	return db
}

func TestRLS_LerPublicacoes_OwnerEAdminVeemTodasDoTenantOsDemaisAsDosSeusCofres(t *testing.T) {
	db := duasPublicacoes(t)

	vistas := []struct {
		ator     identity.Actor
		esperado int64
	}{{donaDoTenant, 2}, {adminDeFora, 2}, {membroDoCofre, 1}, {leitorDoCofre, 1}, {donoDeOutro, 0}}
	for _, v := range vistas {
		ator, esperado := v.ator, v.esperado
		n, err := comoPapelDaAplicacao(t, db, ator, "SELECT count(*) FROM publications")
		require.NoError(t, err, ator.UserID)
		assert.Equal(t, esperado, n, ator.UserID)
		n, err = comoPapelDaAplicacao(t, db, ator, "SELECT count(*) FROM markupp_list_publications('', '')")
		require.NoError(t, err, ator.UserID)
		assert.Equal(t, esperado, n, "lista de %s", ator.UserID)
	}
}

func TestRLS_ListaDePublicacoes_TrazONomeDoCofreForaDoAlcanceDoAdmin(t *testing.T) {
	db := duasPublicacoes(t)

	n, err := comoPapelDaAplicacao(t, db, adminDeFora,
		"SELECT count(*) FROM markupp_list_publications('va', '') WHERE vault_name = 'va' AND folder_path = 'docs' AND note_count = 1")

	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestRLS_Despublicar_AdminTudoDoTenantMembroSoANotaDoSeuCofre(t *testing.T) {
	casos := []struct {
		ator       identity.Actor
		publicacao string
		apaga      int64
	}{
		{adminDeFora, "p-docs", 1},
		{donaDoTenant, "p-nb", 1},
		{membroDoCofre, "p-docs", 0},
		{membroDoCofre, "p-nb", 0},
		{leitorDoCofre, "p-docs", 0},
		{donoDeOutro, "p-docs", 0},
	}
	for _, c := range casos {
		db := duasPublicacoes(t)
		for _, sql := range []string{
			"WITH d AS (DELETE FROM publications WHERE id = '" + c.publicacao + "' RETURNING 1) SELECT count(*) FROM d",
			"WITH u AS (UPDATE publications SET indexable = true WHERE id = '" + c.publicacao + "' RETURNING 1) SELECT count(*) FROM u",
		} {
			n, err := comoPapelDaAplicacao(t, db, c.ator, sql)
			require.NoError(t, err, sql)
			assert.Equal(t, c.apaga, n, "%s em %s: %s", c.ator.UserID, c.publicacao, sql)
		}
	}
}

func TestRLS_Despublicar_MembroDespublicaANotaQuePodeEditar(t *testing.T) {
	db := tenantComDoisCofres(t)
	publica(t, db, "p-na", "t1", "va", alvoNota("na"))

	n, err := comoPapelDaAplicacao(t, db, membroDoCofre, "WITH d AS (DELETE FROM publications WHERE id = 'p-na' RETURNING 1) SELECT count(*) FROM d")

	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestRLS_PapelDaAplicacao_NaoEscreveNasNotasPublicasELeSoAsDosSeusCofres(t *testing.T) {
	db := duasPublicacoes(t)

	n, err := comoPapelDaAplicacao(t, db, membroDoCofre, "SELECT count(*) FROM public_notes")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	_, err = comoPapelDaAplicacao(t, db, donaDoTenant, "WITH d AS (DELETE FROM public_notes RETURNING 1) SELECT count(*) FROM d")
	requirePermissaoNegada(t, err, "aplicacao apagando nota publica")
	_, err = comoPapelDaAplicacao(t, db, donaDoTenant, "WITH u AS (UPDATE publications SET kind = 'vault' RETURNING 1) SELECT count(*) FROM u")
	requirePermissaoNegada(t, err, "aplicacao trocando o alvo da publicacao")
}
