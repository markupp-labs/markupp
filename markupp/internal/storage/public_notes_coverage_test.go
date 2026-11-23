package storage_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

var formatoDoIDPublico = regexp.MustCompile(`^[A-Za-z0-9]{12}$`)

// alvoDePublicacao é o alvo gravado direto na tabela: kind e, conforme ele, a nota ou a
// pasta.
type alvoDePublicacao struct {
	kind, noteID, folder string
}

func alvoNota(noteID string) alvoDePublicacao { return alvoDePublicacao{kind: "note", noteID: noteID} }
func alvoPasta(folder string) alvoDePublicacao {
	return alvoDePublicacao{kind: "folder", folder: folder}
}
func alvoCofre() alvoDePublicacao { return alvoDePublicacao{kind: "vault"} }
func notaEm(vaultID, id, path string) notes.Note {
	return notaPublica(id, path, vaultID, "conteudo de "+id)
}

// textoOuNulo grava o texto vazio como NULL.
func textoOuNulo(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// publica grava, com o papel dono do banco, a publicação id do alvo no cofre vaultID
// do tenant tenantID. O gatilho cobre as notas na mesma instrução.
func publica(t *testing.T, db *pgxpool.Pool, id, tenantID, vaultID string, alvo alvoDePublicacao) {
	t.Helper()
	_, err := db.Exec(context.Background(), `INSERT INTO publications (id, tenant_id, vault_id, kind, note_id, folder_path, published_by, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'quem-publicou', now())`,
		id, tenantID, vaultID, alvo.kind, textoOuNulo(alvo.noteID), textoOuNulo(alvo.folder))
	require.NoError(t, err, "publicar %+v como %q", alvo, id)
}

func despublica(t *testing.T, db *pgxpool.Pool, id string) {
	t.Helper()
	_, err := db.Exec(context.Background(), "DELETE FROM publications WHERE id = $1", id)
	require.NoError(t, err, "despublicar %q", id)
}

// coberturaDe devolve o id público e a publicação da nota, ou vazio quando ela não é
// pública.
func coberturaDe(t *testing.T, db *pgxpool.Pool, noteID string) (publicID, publicationID string) {
	t.Helper()
	err := db.QueryRow(context.Background(), "SELECT public_id, publication_id FROM public_notes WHERE note_id = $1", noteID).
		Scan(&publicID, &publicationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ""
	}
	require.NoError(t, err)
	return publicID, publicationID
}

// cofreComPastas semeia em "va", do tenant t1, notas dentro e fora da pasta docs.
func cofreComPastas(t *testing.T) (*pgxpool.Pool, context.Context, *storage.PostgresNotesRepository) {
	t.Helper()
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "owner"}, "va")
	for _, n := range []notes.Note{
		notaEm("va", "a", "docs/a.md"), notaEm("va", "b", "docs/x/b.md"),
		notaEm("va", "c", "docs2/c.md"), notaEm("va", "d", "d.md"),
	} {
		storagetest.SaveNote(ctx, t, repo, n)
	}
	return db, ctx, repo
}

func TestCobertura_PastaPublicada_CobreSoAsNotasDentroDela(t *testing.T) {
	db, _, _ := cofreComPastas(t)

	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))

	for nota, coberta := range map[string]bool{"a": true, "b": true, "c": false, "d": false} {
		publicID, publicacao := coberturaDe(t, db, nota)
		assert.Equal(t, coberta, publicID != "", "nota %s", nota)
		if coberta {
			assert.Regexp(t, formatoDoIDPublico, publicID)
			assert.Equal(t, "p-docs", publicacao)
		}
	}
}

func TestCobertura_IDsPublicosSaoDistintosPorNota(t *testing.T) {
	db, _, _ := cofreComPastas(t)

	publica(t, db, "p-cofre", "t1", "va", alvoCofre())

	assert.Equal(t, 4, countRows(t, db, "SELECT count(DISTINCT public_id) FROM public_notes"))
}

func TestCobertura_Sobreposicao_NotaPertenceAPublicacaoMaisAmpla(t *testing.T) {
	db, _, _ := cofreComPastas(t)
	publica(t, db, "p-nota", "t1", "va", alvoNota("b"))
	idAntes, _ := coberturaDe(t, db, "b")
	publica(t, db, "p-sub", "t1", "va", alvoPasta("docs/x"))
	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))

	idDepois, publicacao := coberturaDe(t, db, "b")

	assert.Equal(t, "p-docs", publicacao, "a pasta de caminho mais curto vence")
	assert.Equal(t, idAntes, idDepois, "a nota coberta guarda o id")
	publica(t, db, "p-cofre", "t1", "va", alvoCofre())
	_, publicacao = coberturaDe(t, db, "b")
	assert.Equal(t, "p-cofre", publicacao, "o cofre vence a pasta")
}

func TestCobertura_DespublicarPasta_MantemANotaComPublicacaoPropriaEOMesmoID(t *testing.T) {
	db, _, _ := cofreComPastas(t)
	publica(t, db, "p-nota", "t1", "va", alvoNota("a"))
	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))
	idAntes, _ := coberturaDe(t, db, "a")

	despublica(t, db, "p-docs")

	idDepois, publicacao := coberturaDe(t, db, "a")
	assert.Equal(t, idAntes, idDepois)
	assert.Equal(t, "p-nota", publicacao)
	semID, _ := coberturaDe(t, db, "b")
	assert.Empty(t, semID, "a nota sem publicacao propria fica privada")
}

func TestCobertura_MoverNotaParaForaDaPasta_TornaPrivada(t *testing.T) {
	db, ctx, repo := cofreComPastas(t)
	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))

	_, err := repo.Update(ctx, "a", "fora/a.md", "conteudo de a", time.Now(), time.Time{}, true)
	require.NoError(t, err)

	publicID, _ := coberturaDe(t, db, "a")
	assert.Empty(t, publicID)
}

func TestCobertura_NotaNovaOuMovidaParaDentroDaPasta_FicaPublica(t *testing.T) {
	db, ctx, repo := cofreComPastas(t)
	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))

	storagetest.SaveNote(ctx, t, repo, notaEm("va", "nova", "docs/nova.md"))
	_, err := repo.Update(ctx, "c", "docs/c.md", "conteudo de c", time.Now(), time.Time{}, true)
	require.NoError(t, err)

	for _, nota := range []string{"nova", "c"} {
		_, publicacao := coberturaDe(t, db, nota)
		assert.Equal(t, "p-docs", publicacao, "nota %s", nota)
	}
}

func TestCobertura_PublicarDeNovo_GeraIDNovo(t *testing.T) {
	db, _, _ := cofreComPastas(t)
	publica(t, db, "p1", "t1", "va", alvoNota("d"))
	idAntigo, _ := coberturaDe(t, db, "d")
	despublica(t, db, "p1")

	publica(t, db, "p2", "t1", "va", alvoNota("d"))

	idNovo, _ := coberturaDe(t, db, "d")
	assert.NotEmpty(t, idNovo)
	assert.NotEqual(t, idAntigo, idNovo)
}

func TestCobertura_ApagarNota_ApagaALinhaPublicaEAPublicacaoDela(t *testing.T) {
	db, ctx, repo := cofreComPastas(t)
	publica(t, db, "p-nota", "t1", "va", alvoNota("d"))

	require.NoError(t, repo.Delete(ctx, "d"))

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM public_notes"))
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM publications"))
}

func TestCobertura_ApagarCofre_ApagaPublicacoesENotasPublicas(t *testing.T) {
	db, _, _ := cofreComPastas(t)
	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))
	publica(t, db, "p-nota", "t1", "va", alvoNota("a"))

	_, err := db.Exec(context.Background(), "DELETE FROM vaults WHERE id = 'va'")

	require.NoError(t, err)
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM public_notes"))
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM publications"))
}

func TestPublicacoes_AlvoRepetido_RecusadoPeloIndiceUnico(t *testing.T) {
	db, _, _ := cofreComPastas(t)
	publica(t, db, "p1", "t1", "va", alvoPasta("docs"))

	_, err := db.Exec(context.Background(), `INSERT INTO publications (id, tenant_id, vault_id, kind, folder_path, published_by, published_at)
		VALUES ('p2', 't1', 'va', 'folder', 'docs', 'x', now())`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "publications_folder_key")
}

func TestPublicacoes_PastaForaDoFormato_Recusada(t *testing.T) {
	db, _, _ := cofreComPastas(t)

	for _, pasta := range []string{"/docs", "docs/", "docs//x", ""} {
		_, err := db.Exec(context.Background(), `INSERT INTO publications (id, tenant_id, vault_id, kind, folder_path, published_by, published_at)
			VALUES ('p', 't1', 'va', 'folder', $1, 'x', now())`, pasta)
		require.Error(t, err, "pasta %q", pasta)
	}
}
