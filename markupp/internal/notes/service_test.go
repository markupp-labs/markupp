package notes_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

const testMaxContentSize = 100

type fakeRepo struct {
	saved   notes.Note
	saveErr error
	called  bool

	updateNote notes.Note
	updateErr  error
	updateArgs struct {
		id             string
		path           string
		content        string
		updatedAt      time.Time
		lastModifiedAt time.Time
		force          bool
		called         bool
	}

	deleteErr    error
	deleteCalled bool
	deletedID    string

	note   notes.Note
	getErr error

	listResult []notes.Note
	listErr    error
	listCalled bool

	searchResult []notes.SearchResult
	searchErr    error
	searchArgs   struct {
		query  string
		offset int32
		limit  int32
		called bool
	}
}

func (f *fakeRepo) Save(ctx context.Context, n notes.Note) error {
	f.called = true
	f.saved = n
	return f.saveErr
}

func (f *fakeRepo) Update(ctx context.Context, id, path, content string, updatedAt, lastModifiedAt time.Time, force bool) (notes.Note, error) {
	f.updateArgs.called = true
	f.updateArgs.id = id
	f.updateArgs.path = path
	f.updateArgs.content = content
	f.updateArgs.updatedAt = updatedAt
	f.updateArgs.lastModifiedAt = lastModifiedAt
	f.updateArgs.force = force
	return f.updateNote, f.updateErr
}

func (f *fakeRepo) Delete(ctx context.Context, id string) error {
	f.deleteCalled = true
	f.deletedID = id
	return f.deleteErr
}

func (f *fakeRepo) GetNoteByID(ctx context.Context, id string) (notes.Note, error) {
	return f.note, f.getErr
}

func (f *fakeRepo) ListNotes(ctx context.Context) ([]notes.Note, error) {
	f.listCalled = true
	return f.listResult, f.listErr
}

func (f *fakeRepo) SearchNotes(ctx context.Context, query string, offset, limit int32) ([]notes.SearchResult, error) {
	f.searchArgs.called = true
	f.searchArgs.query = query
	f.searchArgs.offset = offset
	f.searchArgs.limit = limit
	return f.searchResult, f.searchErr
}

func newServiceForTest(repo notes.Repository) *notes.Service {
	return notes.NewService(repo, testMaxContentSize)
}

func TestCreate_PathVazio_RetornaErrInvalidPath(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)

	_, err := svc.Create(context.Background(), "", "qualquer")

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrInvalidPath))
	assert.False(t, repo.called)
}

func TestCreate_PathComTraversal_RetornaErrInvalidPath(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)

	_, err := svc.Create(context.Background(), "../etc/passwd", "qualquer")

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrInvalidPath))
}

func TestCreate_PathLongoDemais_RetornaErrInvalidPath(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)
	longPath := strings.Repeat("a", 1025)

	_, err := svc.Create(context.Background(), longPath, "qualquer")

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrInvalidPath))
}

func TestCreate_ContentMuitoGrande_RetornaErrInvalidContent(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)
	bigContent := strings.Repeat("x", testMaxContentSize+1)

	_, err := svc.Create(context.Background(), "ok.md", bigContent)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrInvalidContent))
}

func TestCreate_ContentVazio_Aceito(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)

	note, err := svc.Create(context.Background(), "vazio.md", "")

	require.NoError(t, err)
	assert.Empty(t, note.Content)
	assert.True(t, repo.called)
}

func TestCreate_CaminhoFeliz_GeraNotaCompleta(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)

	before := time.Now().Truncate(time.Millisecond)
	note, err := svc.Create(context.Background(), "minha.md", "conteudo")
	after := time.Now()

	require.NoError(t, err)
	assert.Equal(t, "minha.md", note.Path)
	assert.Equal(t, "conteudo", note.Content)
	assert.NotEmpty(t, note.ID)
	assert.False(t, note.CreatedAt.Before(before))
	assert.False(t, note.CreatedAt.After(after))
	assert.Equal(t, note.CreatedAt, note.UpdatedAt)

	require.True(t, repo.called)
	assert.Equal(t, note, repo.saved)
}

func TestCreate_NormalizaTimestampParaUTCMilissegundo(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)

	got, err := svc.Create(context.Background(), "a.md", "conteudo")

	require.NoError(t, err)
	assert.Equal(t, "UTC", got.CreatedAt.Location().String())
	assert.Equal(t, "UTC", got.UpdatedAt.Location().String())
	assert.True(t, got.UpdatedAt.Equal(got.UpdatedAt.Truncate(time.Millisecond)))
	assert.Equal(t, got.CreatedAt, got.UpdatedAt)
}

func TestCreate_RepoRetornaErrDuplicatePath_PropagadoAoCaller(t *testing.T) {
	repo := &fakeRepo{saveErr: notes.ErrDuplicatePath}
	svc := newServiceForTest(repo)

	_, err := svc.Create(context.Background(), "dup.md", "x")

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrDuplicatePath))
}

func TestUpdate_IDVazio_RetornaErrInvalidID(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)
	now := time.Now()

	_, err := svc.Update(context.Background(), "", "ok.md", "x", now, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrInvalidID))
	assert.False(t, repo.updateArgs.called)
}

func TestUpdate_PathInvalido_RetornaErrInvalidPath(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)
	now := time.Now()

	_, err := svc.Update(context.Background(), "id-1", "", "x", now, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrInvalidPath))
	assert.False(t, repo.updateArgs.called)
}

func TestUpdate_ContentMuitoGrande_RetornaErrInvalidContent(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)
	bigContent := strings.Repeat("x", testMaxContentSize+1)
	now := time.Now()

	_, err := svc.Update(context.Background(), "id-1", "ok.md", bigContent, now, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrInvalidContent))
	assert.False(t, repo.updateArgs.called)
}

func TestUpdate_RepoRetornaErrNotFound_Propagado(t *testing.T) {
	repo := &fakeRepo{updateErr: notes.ErrNotFound}
	svc := newServiceForTest(repo)
	now := time.Now()

	_, err := svc.Update(context.Background(), "id-x", "ok.md", "x", now, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrNotFound))
}

func TestUpdate_RepoRetornaErrDuplicatePath_Propagado(t *testing.T) {
	original := notes.Note{
		ID:        "id-1",
		Path:      "original.md",
		Content:   "antigo",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
	}
	repo := &fakeRepo{
		note:      original,
		updateErr: notes.ErrDuplicatePath,
	}
	svc := newServiceForTest(repo)

	_, err := svc.Update(context.Background(), "id-1", "ok.md", "x", original.UpdatedAt, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrDuplicatePath))
}

func TestUpdate_CaminhoFeliz_RetornaNotaAtualizada(t *testing.T) {
	updated := notes.Note{
		ID:        "id-1",
		Path:      "novo.md",
		Content:   "novo",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
	}
	original := notes.Note{
		ID:        "id-1",
		Path:      "novo.md",
		Content:   "antigo",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
	}
	repo := &fakeRepo{updateNote: updated, note: original}
	svc := newServiceForTest(repo)

	got, err := svc.Update(context.Background(), "id-1", "novo.md", "novo", original.UpdatedAt, false)

	require.NoError(t, err)
	assert.Equal(t, updated, got)
	assert.True(t, repo.updateArgs.called)
	assert.Equal(t, "id-1", repo.updateArgs.id)
	assert.Equal(t, "novo.md", repo.updateArgs.path)
	assert.Equal(t, "novo", repo.updateArgs.content)
	assert.Equal(t, original.UpdatedAt, repo.updateArgs.lastModifiedAt)
	assert.False(t, repo.updateArgs.force)
}

func TestUpdate_ConflictoPorVersao_Force_False_RetornaErrConflict(t *testing.T) {
	clientVersion := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)

	repo := &fakeRepo{updateErr: notes.ErrConflict}
	svc := newServiceForTest(repo)

	_, err := svc.Update(context.Background(), "id-1", "test.md", "novo", clientVersion, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrConflict))
	assert.True(t, repo.updateArgs.called)
}

func TestUpdate_ConflictoPorVersao_Force_True_Sucesso(t *testing.T) {
	currentVersion := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	clientVersion := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)

	nota := notes.Note{
		ID:        "id-1",
		Path:      "test.md",
		Content:   "server version",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: currentVersion,
	}
	updated := notes.Note{
		ID:        "id-1",
		Path:      "test.md",
		Content:   "novo",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Now(),
	}
	repo := &fakeRepo{note: nota, updateNote: updated}
	svc := newServiceForTest(repo)

	got, err := svc.Update(context.Background(), "id-1", "test.md", "novo", clientVersion, true)

	require.NoError(t, err)
	assert.Equal(t, updated, got)
	assert.True(t, repo.updateArgs.force)
}

func TestDelete_IDVazio_RetornaErrInvalidIDSemChamarRepo(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)

	err := svc.Delete(context.Background(), "")

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrInvalidID))
	assert.False(t, repo.deleteCalled)
}

func TestDelete_RepoRetornaErrNotFound_Propagado(t *testing.T) {
	repo := &fakeRepo{deleteErr: notes.ErrNotFound}
	svc := newServiceForTest(repo)

	err := svc.Delete(context.Background(), "id-x")

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrNotFound))
}

func TestDelete_CaminhoFeliz_DelegaParaRepo(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)

	err := svc.Delete(context.Background(), "id-1")

	require.NoError(t, err)
	assert.True(t, repo.deleteCalled)
	assert.Equal(t, "id-1", repo.deletedID)
}

func TestGetNoteByID_IDVazio_RetornaErrInvalidID(t *testing.T) {
	repo := &fakeRepo{}
	svc := newServiceForTest(repo)

	_, err := svc.GetNoteByID(context.Background(), "")

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrInvalidID))
}

func TestGetNoteByID_IDNaoEncontrado_RetornaErrNotFound(t *testing.T) {
	repo := &fakeRepo{getErr: notes.ErrNotFound}
	svc := newServiceForTest(repo)

	_, err := svc.GetNoteByID(context.Background(), "id-inexistente")

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrNotFound))
}

func TestGetNoteByID_CaminhoFeliz_RetornaNotaCorreta(t *testing.T) {
	notaEsperada := notes.Note{
		ID:        "id-123",
		Path:      "arquivo.md",
		Content:   "conteudo",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo := &fakeRepo{note: notaEsperada}
	svc := newServiceForTest(repo)

	nota, err := svc.GetNoteByID(context.Background(), "id-123")

	require.NoError(t, err)
	assert.Equal(t, notaEsperada, nota)
}

func TestListNotes_DelegaParaRepo_RetornaLista(t *testing.T) {
	esperadas := []notes.Note{
		{ID: "id-1", Path: "a.md", Content: "1"},
		{ID: "id-2", Path: "b.md", Content: "2"},
	}
	repo := &fakeRepo{listResult: esperadas}
	svc := newServiceForTest(repo)

	got, err := svc.ListNotes(context.Background())

	require.NoError(t, err)
	assert.True(t, repo.listCalled)
	assert.Equal(t, esperadas, got)
}

func TestListNotes_RepoRetornaErro_Propagado(t *testing.T) {
	repo := &fakeRepo{listErr: errors.New("falha no repo")}
	svc := newServiceForTest(repo)

	_, err := svc.ListNotes(context.Background())

	require.Error(t, err)
}

func TestListNotes_DBVazio_RetornaSliceVazio(t *testing.T) {
	repo := &fakeRepo{listResult: []notes.Note{}}
	svc := newServiceForTest(repo)

	got, err := svc.ListNotes(context.Background())

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSearch_RepoRetornaErro_Propagado(t *testing.T) {
	repo := &fakeRepo{searchErr: errors.New("erro ao buscar")}
	svc := newServiceForTest(repo)

	_, err := svc.SearchNotes(context.Background(), "golang", 0, 10)

	require.Error(t, err)
	assert.True(t, repo.searchArgs.called)
}

func TestSearch_SemResultados_RetornaSliceVazio(t *testing.T) {
	repo := &fakeRepo{searchResult: []notes.SearchResult{}}
	svc := newServiceForTest(repo)

	results, err := svc.SearchNotes(context.Background(), "golang", 0, 10)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearch_ComResultados_RetornaSomenteIdPathUpdatedAt(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepo{searchResult: []notes.SearchResult{
		{ID: "id-1", Path: "golang.md", UpdatedAt: now},
		{ID: "id-2", Path: "tips.md", UpdatedAt: now},
	}}
	svc := newServiceForTest(repo)

	results, err := svc.SearchNotes(context.Background(), "golang", 0, 10)

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "id-1", results[0].ID)
	assert.Equal(t, "golang.md", results[0].Path)
	assert.Equal(t, now, results[0].UpdatedAt)
	assert.Equal(t, "id-2", results[1].ID)
	assert.Equal(t, "tips.md", results[1].Path)
	assert.Equal(t, now, results[1].UpdatedAt)
}

func TestSearch_PaginacaoComOffset_PassaParametrosCorretosAoRepo(t *testing.T) {
	now := time.Now()
	repo := &fakeRepo{searchResult: []notes.SearchResult{
		{ID: "id-2", Path: "b.md", UpdatedAt: now},
		{ID: "id-3", Path: "c.md", UpdatedAt: now},
	}}
	svc := newServiceForTest(repo)

	results, err := svc.SearchNotes(context.Background(), "golang", 1, 2)

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "id-2", results[0].ID)
	assert.Equal(t, "id-3", results[1].ID)
	assert.True(t, repo.searchArgs.called)
	assert.Equal(t, "golang", repo.searchArgs.query)
	assert.Equal(t, int32(1), repo.searchArgs.offset)
	assert.Equal(t, int32(2), repo.searchArgs.limit)
}

func TestSearch_LimitZero_UsaPadraoDeZ(t *testing.T) {
	repo := &fakeRepo{searchResult: make([]notes.SearchResult, 10)}
	svc := newServiceForTest(repo)

	_, err := svc.SearchNotes(context.Background(), "golang", 0, 0)

	require.NoError(t, err)
	assert.Equal(t, int32(10), repo.searchArgs.limit)
}

func TestSearch_LimitNegativo_UsaPadraoDeZ(t *testing.T) {
	repo := &fakeRepo{searchResult: make([]notes.SearchResult, 10)}
	svc := newServiceForTest(repo)

	_, err := svc.SearchNotes(context.Background(), "golang", 0, -5)

	require.NoError(t, err)
	assert.Equal(t, int32(10), repo.searchArgs.limit)
}

func TestSearch_OffsetNegativo_UsaZero(t *testing.T) {
	repo := &fakeRepo{searchResult: []notes.SearchResult{}}
	svc := newServiceForTest(repo)

	_, err := svc.SearchNotes(context.Background(), "golang", -10, 5)

	require.NoError(t, err)
	assert.Equal(t, int32(0), repo.searchArgs.offset)
}

func TestSearch_PassaQueryCruaAoRepo(t *testing.T) {
	repo := &fakeRepo{searchResult: []notes.SearchResult{}}
	svc := newServiceForTest(repo)

	_, err := svc.SearchNotes(context.Background(), "test", 0, 10)
	require.NoError(t, err)

	assert.Equal(t, "test", repo.searchArgs.query)
}

func TestSearch_QueryVazia_PassaVaziaAoRepo(t *testing.T) {
	repo := &fakeRepo{searchResult: []notes.SearchResult{}}
	svc := newServiceForTest(repo)

	_, err := svc.SearchNotes(context.Background(), "", 0, 10)
	require.NoError(t, err)

	assert.Equal(t, "", repo.searchArgs.query)
}
