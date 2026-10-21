package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// indexaTodosOsCofres esvazia a fila do indexador com a configuração de texto config.
func indexaTodosOsCofres(t *testing.T, db *pgxpool.Pool, config string) {
	t.Helper()
	idx := novoIndexador(t, db, config)
	for {
		if found, _ := indexaProximo(t, idx); !found {
			return
		}
	}
}

func notaComCaminho(id, path, content string) notes.Note {
	agora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	return notes.Note{ID: id, VaultID: cofreDeTeste, Path: path, Content: content, CreatedAt: agora, UpdatedAt: agora}
}

// cofreBuscavel salva as notas em cofreDeTeste, indexa com config e devolve o
// repositório que busca com a mesma config.
func cofreBuscavel(t *testing.T, config string, lista ...notes.Note) *storage.PostgresNotesRepository {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, config)
	for _, nota := range lista {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, config)
	return repo
}

func idsDaBusca(ctx context.Context, t *testing.T, repo *storage.PostgresNotesRepository, query string) []string {
	t.Helper()
	achadas, err := repo.SearchNotes(ctx, query, "", 0, 10)
	require.NoError(t, err, "busca %q", query)
	ids := make([]string, 0, len(achadas))
	for _, achada := range achadas {
		ids = append(ids, achada.ID)
	}
	return ids
}

func TestSearchNotes_TituloPesaMaisQueCorpo(t *testing.T) {
	repo := cofreBuscavel(t, "simple",
		notaComCaminho("corpo", "cluster.md", "nota sobre kubernetes"),
		notaComCaminho("titulo", "kubernetes.md", "nota sobre cluster"))

	assert.Equal(t, []string{"titulo", "corpo"}, idsDaBusca(ctxDeTeste, t, repo, "kubernetes"))
}

func TestSearchNotes_EmpateNoRanking_DesempataPeloID(t *testing.T) {
	repo := cofreBuscavel(t, "simple",
		notaComCaminho("b", "b.md", "golang"),
		notaComCaminho("a", "a.md", "golang"),
		notaComCaminho("c", "c.md", "golang"))

	assert.Equal(t, []string{"a", "b", "c"}, idsDaBusca(ctxDeTeste, t, repo, "golang"))
}

func TestSearchNotes_SintaxeWebsearch(t *testing.T) {
	repo := cofreBuscavel(t, "simple",
		notaComCaminho("ambos", "x1.md", "go e rust"),
		notaComCaminho("go", "x2.md", "go puro"),
		notaComCaminho("rust", "x3.md", "rust puro"),
		notaComCaminho("frase", "x4.md", "frase exata aqui"),
		notaComCaminho("invertida", "x5.md", "exata frase"))

	assert.ElementsMatch(t, []string{"frase"}, idsDaBusca(ctxDeTeste, t, repo, `"frase exata"`), "aspas pedem a frase")
	assert.ElementsMatch(t, []string{"ambos", "go", "rust"}, idsDaBusca(ctxDeTeste, t, repo, "go OR rust"))
	assert.ElementsMatch(t, []string{"go"}, idsDaBusca(ctxDeTeste, t, repo, "go -rust"), "menos exclui o termo")
	assert.ElementsMatch(t, []string{"ambos"}, idsDaBusca(ctxDeTeste, t, repo, "go rust"), "termos soltos exigem todos")
}

func TestSearchNotes_DevolveTrechoComOTermoMarcado(t *testing.T) {
	repo := cofreBuscavel(t, "simple", notaComCaminho("n1", "n1.md", "o kubernetes orquestra containers"))

	achadas, err := repo.SearchNotes(ctxDeTeste, "kubernetes", "", 0, 10)

	require.NoError(t, err)
	require.Len(t, achadas, 1)
	assert.Contains(t, achadas[0].Snippet, "\u0002kubernetes\u0003")
}

func TestSearchNotes_TrechoSemHTML_DevolveTextoCruComMarcadoresNeutros(t *testing.T) {
	repo := cofreBuscavel(t, "simple", notaComCaminho("n1", "n1.md", "<script>alert(1)</script> kubernetes"))

	achadas, err := repo.SearchNotes(ctxDeTeste, "kubernetes", "", 0, 10)

	require.NoError(t, err)
	require.Len(t, achadas, 1)
	assert.Contains(t, achadas[0].Snippet, "\u0002kubernetes\u0003")
	assert.NotContains(t, achadas[0].Snippet, "<b>", "o servidor nao gera tag")
	assert.NotContains(t, achadas[0].Snippet, "</b>", "o servidor nao gera tag")
}

func TestSearchNotes_FiltraPeloCofreEDevolveARelevancia(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctxOutro := storagetest.SeedMember(t, db, membroDeTeste, "outro-cofre")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("titulo", "kubernetes.md", "cluster"))
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("corpo", "c.md", "sobre kubernetes"))
	fora := notaComCaminho("fora", "k.md", "kubernetes")
	fora.VaultID = "outro-cofre"
	storagetest.SaveNote(ctxOutro, t, repo, fora)
	indexaTodosOsCofres(t, db, "simple")

	noCofre, err := repo.SearchNotes(ctxDeTeste, "kubernetes", cofreDeTeste, 0, 10)
	require.NoError(t, err)
	todas, err := repo.SearchNotes(ctxDeTeste, "kubernetes", "", 0, 10)
	require.NoError(t, err)

	require.Len(t, noCofre, 2)
	assert.Equal(t, []string{"titulo", "corpo"}, []string{noCofre[0].ID, noCofre[1].ID})
	assert.Greater(t, noCofre[0].Rank, noCofre[1].Rank, "o titulo pesa mais")
	assert.Positive(t, noCofre[1].Rank)
	assert.Len(t, todas, 3, "sem cofre, busca em todos os do ator")
}

func TestSearchNotes_CofreInvisivel_ErrVaultForbidden(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SeedMember(t, db, identity.Actor{UserID: "u-fora", TenantID: "outro", Role: "admin"}, "cofre-fora")

	_, err := repo.SearchNotes(ctxDeTeste, "kubernetes", "cofre-fora", 0, 10)

	require.ErrorIs(t, err, notes.ErrVaultForbidden)
}

func TestSearchNotes_PorcentoESublinhado_NaoSaoCuringa(t *testing.T) {
	repo := cofreBuscavel(t, "simple",
		notaComCaminho("n1", "n1.md", "qualquer coisa"),
		notaComCaminho("n2", "n2.md", "outra coisa"))

	for _, query := range []string{"%", "_", "%oisa", "c_isa"} {
		assert.Empty(t, idsDaBusca(ctxDeTeste, t, repo, query), "busca %q", query)
	}
}

func TestSearchNotes_ConsultaVaziaOuSoStopwords_DevolveVazioSemErro(t *testing.T) {
	repo := cofreBuscavel(t, "portuguese", notaComCaminho("n1", "n1.md", "a casa de pedra"))

	for _, query := range []string{"", "   ", "de", "a de o"} {
		assert.Empty(t, idsDaBusca(ctxDeTeste, t, repo, query), "busca %q", query)
	}
	assert.Equal(t, []string{"n1"}, idsDaBusca(ctxDeTeste, t, repo, "casas"), "a config injetada radicaliza a consulta")
}

func TestSearchNotes_TituloCitadoEmWikilink_ViraTexto(t *testing.T) {
	repo := cofreBuscavel(t, "simple", notaComCaminho("n1", "n1.md", "veja [[Kubernetes|k8s]] depois"))

	assert.Equal(t, []string{"n1"}, idsDaBusca(ctxDeTeste, t, repo, "kubernetes"))
	assert.Equal(t, []string{"n1"}, idsDaBusca(ctxDeTeste, t, repo, "k8s"))
}

func TestSearchNotes_NotaAindaNaoIndexada_NaoAparece(t *testing.T) {
	repo := storage.NewPostgresNotesRepository(setupNotesDB(t), "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n1", "n1.md", "golang"))

	assert.Empty(t, idsDaBusca(ctxDeTeste, t, repo, "golang"), "consistencia eventual: so o indexador poe a nota na busca")
}

func TestSearchNotes_IsolaPorCofreETenant(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctxA := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	ctxVizinho := storagetest.SeedMember(t, db, identity.Actor{UserID: "uv", TenantID: "t1", Role: "member"}, "vv")
	ctxB := storagetest.SeedMember(t, db, identity.Actor{UserID: "ub", TenantID: "t2", Role: "member"}, "vb")
	storagetest.SaveNote(ctxA, t, repo, notaComConteudo("a1", "va", "segredo de a"))
	storagetest.SaveNote(ctxVizinho, t, repo, notaComConteudo("v1", "vv", "segredo do vizinho"))
	storagetest.SaveNote(ctxB, t, repo, notaComConteudo("b1", "vb", "segredo de b"))
	indexaTodosOsCofres(t, db, "simple")

	assert.Equal(t, []string{"a1"}, idsDaBusca(ctxA, t, repo, "segredo"))
	assert.Equal(t, []string{"v1"}, idsDaBusca(ctxVizinho, t, repo, "segredo"), "outro cofre do mesmo tenant fica fora")
	assert.Equal(t, []string{"b1"}, idsDaBusca(ctxB, t, repo, "segredo"), "outro tenant fica fora")
}

func TestSearchNotes_ViewerLeOIndiceDoCofre(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctxMembro := storagetest.SeedMember(t, db, identity.Actor{UserID: "um", TenantID: "t1", Role: "member"}, "va")
	ctxViewer := storagetest.SeedMember(t, db, identity.Actor{UserID: "uv", TenantID: "t1", Role: "viewer"}, "va")
	storagetest.SaveNote(ctxMembro, t, repo, notaComConteudo("n1", "va", "relatorio anual"))
	indexaTodosOsCofres(t, db, "simple")

	assert.Equal(t, []string{"n1"}, idsDaBusca(ctxViewer, t, repo, "relatorio"))
}

func TestVaultIndexer_DocumentoPoeOTituloComPesoAEOCorpoComPesoD(t *testing.T) {
	db := setupNotesDB(t)
	storagetest.SaveNote(ctxDeTeste, t, storage.NewPostgresNotesRepository(db, "simple"),
		notaComCaminho("n1", "pasta/Kubernetes.md", "orquestra containers"))

	indexaTodosOsCofres(t, db, "simple")

	assert.Equal(t, 1, casaNoIndice(t, db, "n1", "simple", "kubernetes:A"), "titulo vem do ultimo segmento do caminho")
	assert.Equal(t, 1, casaNoIndice(t, db, "n1", "simple", "containers:D"))
	assert.Zero(t, casaNoIndice(t, db, "n1", "simple", "pasta"), "pastas do caminho nao entram no documento")
}

func TestMigracao00019_ReenfileiraTodasAsNotasParaODocumentoNovo(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.UpTo(context.Background(), 18)
	require.NoError(t, err)
	storagetest.SeedMember(t, pool, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	storagetest.SeedMember(t, pool, identity.Actor{UserID: "ub", TenantID: "t2", Role: "member"}, "vb")
	_, err = pool.Exec(context.Background(), `INSERT INTO notes (id, tenant_id, vault_id, path, content, created_at, updated_at)
		VALUES ('a', 't1', 'va', 'a.md', 'oi', now(), now()), ('b', 't2', 'vb', 'b.md', 'oi', now(), now())`)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), "DELETE FROM note_index_queue")
	require.NoError(t, err)

	_, err = provider.UpTo(context.Background(), 19)

	require.NoError(t, err)
	assert.Equal(t, 2, countRows(t, pool, consultaDoScaler), "os cofres de todos os tenants voltam a ser indexados")
}

func TestMigracao00019_DownEUp(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.Up(context.Background())
	require.NoError(t, err)

	_, err = provider.DownTo(context.Background(), 18)
	require.NoError(t, err)
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
}
