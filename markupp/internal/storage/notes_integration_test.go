package storage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

func TestSearchNotes_ComResultados_RetornaPaginado(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctx := ctxDeTeste

	now := time.Now()
	notesData := []notes.Note{
		{ID: "1", VaultID: cofreDeTeste, Path: "golang1.md", Content: "golang tutorial", CreatedAt: now, UpdatedAt: now},
		{ID: "2", VaultID: cofreDeTeste, Path: "golang2.md", Content: "golang tips", CreatedAt: now, UpdatedAt: now},
		{ID: "3", VaultID: cofreDeTeste, Path: "python.md", Content: "python guide", CreatedAt: now, UpdatedAt: now},
		{ID: "4", VaultID: cofreDeTeste, Path: "golang3.md", Content: "golang advanced", CreatedAt: now, UpdatedAt: now},
	}
	for _, note := range notesData {
		_, err := repo.Save(ctx, note)
		require.NoError(t, err)
	}

	indexaTodosOsCofres(t, db, "simple")
	results, err := repo.SearchNotes(ctx, "golang", "", 0, 10)

	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "1", results[0].ID)
	assert.Equal(t, "golang1.md", results[0].Path)
	assert.Equal(t, now.Unix(), results[0].UpdatedAt.Unix())
}

func TestSearchNotes_ComPaginacao_RetornaApenasLimitAndOffset(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctx := ctxDeTeste

	now := time.Now()
	for i := 1; i <= 5; i++ {
		note := notes.Note{
			ID:        string(rune('0' + i)),
			VaultID:   cofreDeTeste,
			Path:      "golang" + string(rune('0'+i)) + ".md",
			Content:   "golang content " + string(rune('0'+i)),
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, err := repo.Save(ctx, note)
		require.NoError(t, err)
	}

	indexaTodosOsCofres(t, db, "simple")
	results, err := repo.SearchNotes(ctx, "golang", "", 1, 2)

	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestSearchNotes_OffsetMaiorQueTotal_RetornaVazio(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctx := ctxDeTeste

	now := time.Now()
	note := notes.Note{
		ID:        "1",
		VaultID:   cofreDeTeste,
		Path:      "golang.md",
		Content:   "golang tutorial",
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := repo.Save(ctx, note)
	require.NoError(t, err)

	indexaTodosOsCofres(t, db, "simple")
	results, err := repo.SearchNotes(ctx, "golang", "", 100, 10)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchNotes_NaoEncontra_RetornaVazio(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctx := ctxDeTeste

	now := time.Now()
	note := notes.Note{
		ID:        "1",
		VaultID:   cofreDeTeste,
		Path:      "python.md",
		Content:   "python tutorial",
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := repo.Save(ctx, note)
	require.NoError(t, err)

	indexaTodosOsCofres(t, db, "simple")
	results, err := repo.SearchNotes(ctx, "golang", "", 0, 10)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchNotes_IgnoraCaixa(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctx := ctxDeTeste

	now := time.Now()
	notesData := []notes.Note{
		{ID: "1", VaultID: cofreDeTeste, Path: "a.md", Content: "Golang Tutorial", CreatedAt: now, UpdatedAt: now},
		{ID: "2", VaultID: cofreDeTeste, Path: "b.md", Content: "golang tips", CreatedAt: now, UpdatedAt: now},
	}
	for _, note := range notesData {
		_, err := repo.Save(ctx, note)
		require.NoError(t, err)
	}

	indexaTodosOsCofres(t, db, "simple")
	results, err := repo.SearchNotes(ctx, "golang", "", 0, 10)

	require.NoError(t, err)
	require.Len(t, results, 2)
}
