package storage_test

import (
	"context"
	"errors"
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

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := storagetest.EmptyPool(t)
	require.NoError(t, storage.Migrate(context.Background(), pool))
	return pool
}

var membroDeTeste = identity.Actor{UserID: "u-teste", TenantID: "default", Role: "admin"}

const cofreDeTeste = "cofre-teste"

var ctxDeTeste = identity.WithActor(context.Background(), membroDeTeste)

// setupNotesDB migra o banco e semeia membroDeTeste em cofreDeTeste.
func setupNotesDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := setupTestDB(t)
	storagetest.SeedMember(t, pool, membroDeTeste, cofreDeTeste)
	return pool
}

func sampleNote() notes.Note {
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	return notes.Note{
		ID:        "id-test-1",
		VaultID:   cofreDeTeste,
		Path:      "amostra.md",
		Content:   "conteudo de amostra",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestPostgresRepo_Save_PersisteCamposCorretos(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	n := sampleNote()

	err := repo.Save(ctxDeTeste, n)
	require.NoError(t, err)

	var gotID, gotPath, gotContent string
	var gotCreated, gotUpdated time.Time
	err = db.QueryRow(ctxDeTeste,
		"SELECT id, path, content, created_at, updated_at FROM notes WHERE id = $1",
		n.ID,
	).Scan(&gotID, &gotPath, &gotContent, &gotCreated, &gotUpdated)
	require.NoError(t, err)

	assert.Equal(t, n.ID, gotID)
	assert.Equal(t, n.Path, gotPath)
	assert.Equal(t, n.Content, gotContent)
	assert.True(t, n.CreatedAt.Equal(gotCreated))
	assert.True(t, n.UpdatedAt.Equal(gotUpdated))
}

func TestPostgresRepo_Save_PathDuplicado_RetornaErrDuplicatePath(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	n1 := sampleNote()
	n2 := sampleNote()
	n2.ID = "id-test-2"

	require.NoError(t, repo.Save(ctxDeTeste, n1))

	err := repo.Save(ctxDeTeste, n2)
	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrDuplicatePath))
}

func TestPostgresRepo_Save_Sucesso_RetornaNil(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)

	err := repo.Save(ctxDeTeste, sampleNote())
	assert.NoError(t, err)
}

func TestPostgresRepo_Update_AtualizaCamposEPreservaCreatedAt(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	original := sampleNote()
	require.NoError(t, repo.Save(ctxDeTeste, original))

	novoUpdatedAt := original.UpdatedAt.Add(time.Hour)
	got, err := repo.Update(ctxDeTeste, original.ID, "renomeado.md", "novo conteudo", novoUpdatedAt, original.UpdatedAt, false)

	require.NoError(t, err)
	assert.Equal(t, original.ID, got.ID)
	assert.Equal(t, "renomeado.md", got.Path)
	assert.Equal(t, "novo conteudo", got.Content)
	assert.True(t, original.CreatedAt.Equal(got.CreatedAt))
	assert.True(t, got.UpdatedAt.Equal(novoUpdatedAt))
}

func TestPostgresRepo_Update_IDInexistente_RetornaErrNotFound(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)

	ts := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	_, err := repo.Update(ctxDeTeste, "nao-existe", "x.md", "y", ts, ts, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrNotFound))
}

func TestPostgresRepo_Update_PathDuplicado_RetornaErrDuplicatePath(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	n1 := sampleNote()
	n2 := sampleNote()
	n2.ID = "id-test-2"
	n2.Path = "outra.md"
	require.NoError(t, repo.Save(ctxDeTeste, n1))
	require.NoError(t, repo.Save(ctxDeTeste, n2))

	novoUpdatedAt := n2.UpdatedAt.Add(time.Hour)
	_, err := repo.Update(ctxDeTeste, n2.ID, n1.Path, n2.Content, novoUpdatedAt, n2.UpdatedAt, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrDuplicatePath))
}

func TestPostgresRepo_Delete_RemoveLinha(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	n := sampleNote()
	require.NoError(t, repo.Save(ctxDeTeste, n))

	err := repo.Delete(ctxDeTeste, n.ID)
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRow(ctxDeTeste, "SELECT COUNT(*) FROM notes WHERE id = $1", n.ID).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestPostgresRepo_Delete_IDInexistente_RetornaErrNotFound(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)

	err := repo.Delete(ctxDeTeste, "nao-existe")

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrNotFound))
}

func TestPostgresRepo_Update_ComVersaoCorreta_Force_False_Sucesso(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	original := sampleNote()
	require.NoError(t, repo.Save(ctxDeTeste, original))

	novoUpdatedAt := original.UpdatedAt.Add(time.Hour)
	got, err := repo.Update(ctxDeTeste, original.ID, "renomeado.md", "novo conteudo", novoUpdatedAt, original.UpdatedAt, false)

	require.NoError(t, err)
	assert.Equal(t, "renomeado.md", got.Path)
	assert.Equal(t, "novo conteudo", got.Content)
}

func TestPostgresRepo_Update_ComVersaoIncorreta_Force_False_RetornaErrConflict(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	original := sampleNote()
	require.NoError(t, repo.Save(ctxDeTeste, original))

	novoUpdatedAt := original.UpdatedAt.Add(time.Hour)
	versaoAnterior := original.UpdatedAt.Add(-1 * time.Second)
	_, err := repo.Update(ctxDeTeste, original.ID, "renomeado.md", "novo conteudo", novoUpdatedAt, versaoAnterior, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrConflict))
}

func TestPostgresRepo_Update_ComVersaoIncorreta_Force_True_Sucesso(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	original := sampleNote()
	require.NoError(t, repo.Save(ctxDeTeste, original))

	novoUpdatedAt := original.UpdatedAt.Add(time.Hour)
	versaoAnterior := original.UpdatedAt.Add(-1 * time.Second)
	got, err := repo.Update(ctxDeTeste, original.ID, "renomeado.md", "novo conteudo", novoUpdatedAt, versaoAnterior, true)

	require.NoError(t, err)
	assert.Equal(t, "renomeado.md", got.Path)
	assert.Equal(t, "novo conteudo", got.Content)
}

func TestPostgresRepo_Update_IDInexistente_Force_False_RetornaErrNotFound(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)

	ts := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	_, err := repo.Update(ctxDeTeste, "nao-existe", "x.md", "y", ts, ts, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrNotFound))
}

func TestPostgresRepo_Update_IDInexistente_Force_True_RetornaErrNotFound(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)

	ts := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	_, err := repo.Update(ctxDeTeste, "nao-existe", "x.md", "y", ts, ts, true)

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrNotFound))
}

func TestPostgresRepo_Update_UsaUpdatedAtRecebido(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	original := sampleNote()
	require.NoError(t, repo.Save(ctxDeTeste, original))

	novoUpdatedAt := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	got, err := repo.Update(ctxDeTeste, original.ID, "novo.md", "novo", novoUpdatedAt, original.UpdatedAt, false)

	require.NoError(t, err)
	assert.True(t, got.UpdatedAt.Equal(novoUpdatedAt))
}

func TestPostgresRepo_ListNotes_DBVazio_RetornaSliceVazio(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)

	got, err := repo.ListNotes(ctxDeTeste)

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPostgresRepo_ListNotes_RetornaTodasNotasOrdenadasPorPath(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)

	notas := []notes.Note{
		{ID: "id-c", VaultID: cofreDeTeste, Path: "c.md", Content: "ccc", CreatedAt: now, UpdatedAt: now},
		{ID: "id-a", VaultID: cofreDeTeste, Path: "a.md", Content: "aaa", CreatedAt: now, UpdatedAt: now},
		{ID: "id-b", VaultID: cofreDeTeste, Path: "b.md", Content: "bbb", CreatedAt: now, UpdatedAt: now},
	}
	for _, n := range notas {
		require.NoError(t, repo.Save(ctxDeTeste, n))
	}

	got, err := repo.ListNotes(ctxDeTeste)

	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "a.md", got[0].Path)
	assert.Equal(t, "b.md", got[1].Path)
	assert.Equal(t, "c.md", got[2].Path)
	assert.Equal(t, "aaa", got[0].Content)
}

func TestPostgresRepo_GetNoteByID_IDInexistente_RetornaErrNotFound(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db)

	_, err := repo.GetNoteByID(ctxDeTeste, "nao-existe")

	require.Error(t, err)
	assert.True(t, errors.Is(err, notes.ErrNotFound))
}
