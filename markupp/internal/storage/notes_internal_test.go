package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

const inserirNotaInterna = "INSERT INTO notes (id, path, content, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)"

func bancoInternoMigrado(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := storagetest.EmptyPool(t)
	require.NoError(t, Migrate(context.Background(), pool))
	return pool
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
	repo := NewPostgresNotesRepository(bancoInternoMigrado(t))
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
	repo := NewPostgresNotesRepository(storagetest.EmptyPool(t))

	_, err := repo.GetNoteByID(context.Background(), "id-interno-1")

	require.Error(t, err)
	assert.False(t, errors.Is(err, notes.ErrNotFound),
		"esperado erro cru do driver, recebido notes.ErrNotFound: %v", err)
}

func TestIsUniqueConstraintViolation_ConstraintUnicaDePath_RetornaTrue(t *testing.T) {
	db := bancoInternoMigrado(t)
	nota := notaInterna()
	_, err := db.Exec(context.Background(), inserirNotaInterna, nota.ID, nota.Path, nota.Content, nota.CreatedAt, nota.UpdatedAt)
	require.NoError(t, err)

	_, err = db.Exec(context.Background(), inserirNotaInterna, "id-interno-2", nota.Path, nota.Content, nota.CreatedAt, nota.UpdatedAt)

	require.Error(t, err)
	assert.True(t, isUniqueConstraintViolation(err),
		"esperado *pgconn.PgError com code %s, recebido %v", uniqueViolationCode, err)
}

func TestIsUniqueConstraintViolation_ConstraintNotNull_RetornaFalse(t *testing.T) {
	db := bancoInternoMigrado(t)
	nota := notaInterna()

	_, err := db.Exec(context.Background(), inserirNotaInterna, nota.ID, nota.Path, nil, nota.CreatedAt, nota.UpdatedAt)

	require.Error(t, err)
	assert.False(t, isUniqueConstraintViolation(err),
		"esperado false para *pgconn.PgError com code diferente de %s, recebido %v", uniqueViolationCode, err)
}

func TestIsUniqueConstraintViolation_ErroForaDoDriver_RetornaFalse(t *testing.T) {
	err := errors.New("falha de rede ao abrir o banco")

	assert.False(t, isUniqueConstraintViolation(err),
		"esperado false para erro que nao e *pgconn.PgError, recebido %v", err)
}
