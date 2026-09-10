package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

const inserirNotaInterna = "INSERT INTO notes (id, path, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?)"

func bancoInternoMigrado(t *testing.T) *sql.DB {
	t.Helper()
	db := bancoInternoVazio(t)
	require.NoError(t, Migrate(db))
	return db
}

func bancoInternoVazio(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func notaInterna() notes.Note {
	instante := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	return notes.Note{
		ID:        "id-interno-1",
		Path:      "interna.md",
		Content:   "conteudo interno",
		CreatedAt: instante,
		UpdatedAt: instante,
	}
}

func TestGetNoteByID_NotaExistente_RetornaCamposPersistidos(t *testing.T) {
	repo := NewSqliteNotesRepository(bancoInternoMigrado(t))
	nota := notaInterna()
	require.NoError(t, repo.Save(context.Background(), nota))

	obtida, err := repo.GetNoteByID(context.Background(), nota.ID)

	require.NoError(t, err)
	assert.Equal(t, nota.ID, obtida.ID)
	assert.Equal(t, nota.Path, obtida.Path)
	assert.Equal(t, nota.Content, obtida.Content)
	assert.True(t, nota.CreatedAt.Equal(obtida.CreatedAt))
	assert.True(t, nota.UpdatedAt.Equal(obtida.UpdatedAt))
}

func TestGetNoteByID_TabelaAusente_PropagaErroSemTraduzirParaNotFound(t *testing.T) {
	repo := NewSqliteNotesRepository(bancoInternoVazio(t))

	_, err := repo.GetNoteByID(context.Background(), "id-interno-1")

	require.Error(t, err)
	assert.False(t, errors.Is(err, notes.ErrNotFound),
		"esperado erro cru do driver, recebido notes.ErrNotFound: %v", err)
}

func TestIsUniqueConstraintViolation_ConstraintUnicaDePath_RetornaTrue(t *testing.T) {
	db := bancoInternoMigrado(t)
	nota := notaInterna()
	_, err := db.Exec(inserirNotaInterna, nota.ID, nota.Path, nota.Content, nota.CreatedAt, nota.UpdatedAt)
	require.NoError(t, err)

	_, err = db.Exec(inserirNotaInterna, "id-interno-2", nota.Path, nota.Content, nota.CreatedAt, nota.UpdatedAt)

	require.Error(t, err)
	assert.True(t, isUniqueConstraintViolation(err),
		"esperado *sqlite.Error com code %d, recebido %v", sqliteUniqueConstraintCode, err)
}

func TestIsUniqueConstraintViolation_ConstraintNotNull_RetornaFalse(t *testing.T) {
	db := bancoInternoMigrado(t)
	nota := notaInterna()

	_, err := db.Exec(inserirNotaInterna, nota.ID, nota.Path, nil, nota.CreatedAt, nota.UpdatedAt)

	require.Error(t, err)
	assert.False(t, isUniqueConstraintViolation(err),
		"esperado false para *sqlite.Error com code diferente de %d, recebido %v", sqliteUniqueConstraintCode, err)
}

func TestIsUniqueConstraintViolation_ErroForaDoDriver_RetornaFalse(t *testing.T) {
	err := errors.New("falha de rede ao abrir o banco")

	assert.False(t, isUniqueConstraintViolation(err),
		"esperado false para erro que nao e *sqlite.Error, recebido %v", err)
}
