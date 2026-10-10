package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/tenant"
)

func TestSearchNotes_ComResultados_RetornaPaginado(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := context.Background()

	now := time.Now()
	notesData := []notes.Note{
		{ID: "1", Path: "golang1.md", Content: "golang tutorial", CreatedAt: now, UpdatedAt: now},
		{ID: "2", Path: "golang2.md", Content: "golang tips", CreatedAt: now, UpdatedAt: now},
		{ID: "3", Path: "python.md", Content: "python guide", CreatedAt: now, UpdatedAt: now},
		{ID: "4", Path: "golang3.md", Content: "golang advanced", CreatedAt: now, UpdatedAt: now},
	}
	for _, note := range notesData {
		err := repo.Save(ctx, note)
		require.NoError(t, err)
	}

	results, err := repo.SearchNotes(ctx, "golang", 0, 10)

	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "1", results[0].ID)
	assert.Equal(t, "golang1.md", results[0].Path)
	assert.Equal(t, now.Unix(), results[0].UpdatedAt.Unix())
}

func TestSearchNotes_ComPaginacao_RetornaApenasLimitAndOffset(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := context.Background()

	now := time.Now()
	for i := 1; i <= 5; i++ {
		note := notes.Note{
			ID:        string(rune('0' + i)),
			Path:      "golang" + string(rune('0'+i)) + ".md",
			Content:   "golang content " + string(rune('0'+i)),
			CreatedAt: now,
			UpdatedAt: now,
		}
		err := repo.Save(ctx, note)
		require.NoError(t, err)
	}

	results, err := repo.SearchNotes(ctx, "golang", 1, 2)

	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestSearchNotes_OffsetMaiorQueTotal_RetornaVazio(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := context.Background()

	now := time.Now()
	note := notes.Note{
		ID:        "1",
		Path:      "golang.md",
		Content:   "golang tutorial",
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := repo.Save(ctx, note)
	require.NoError(t, err)

	results, err := repo.SearchNotes(ctx, "golang", 100, 10)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchNotes_NaoEncontra_RetornaVazio(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := context.Background()

	now := time.Now()
	note := notes.Note{
		ID:        "1",
		Path:      "python.md",
		Content:   "python tutorial",
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := repo.Save(ctx, note)
	require.NoError(t, err)

	results, err := repo.SearchNotes(ctx, "golang", 0, 10)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchNotes_LikeEhCaseInsensitive(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := context.Background()

	now := time.Now()
	notesData := []notes.Note{
		{ID: "1", Path: "a.md", Content: "Golang Tutorial", CreatedAt: now, UpdatedAt: now},
		{ID: "2", Path: "b.md", Content: "golang tips", CreatedAt: now, UpdatedAt: now},
	}
	for _, note := range notesData {
		err := repo.Save(ctx, note)
		require.NoError(t, err)
	}

	results, err := repo.SearchNotes(ctx, "golang", 0, 10)

	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestSearchNotes_QueryParcial_CasaSubstring(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)
	ctx := context.Background()

	now := time.Now()
	require.NoError(t, repo.Save(ctx, notes.Note{
		ID: "1", Path: "g.md", Content: "golang tutorial", CreatedAt: now, UpdatedAt: now,
	}))

	results, err := repo.SearchNotes(ctx, "olang", 0, 10)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "1", results[0].ID)
}

func TestTenantIsolation_NotesEntreTenantsDistintos_NaoVazam(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)

	ctxA := tenant.WithContext(context.Background(), tenant.Context{TenantID: "tenant-alpha"})
	ctxB := tenant.WithContext(context.Background(), tenant.Context{TenantID: "tenant-beta"})

	now := time.Now().UTC().Truncate(time.Millisecond)
	noteA := notes.Note{
		ID:        "id-a",
		TenantID:  "tenant-alpha",
		VaultID:   "vault-1",
		Path:      "segredo-alpha.md",
		Content:   "conteudo confidencial do cliente alpha",
		CreatedAt: now,
		UpdatedAt: now,
	}
	noteB := notes.Note{
		ID:        "id-b",
		TenantID:  "tenant-beta",
		VaultID:   "vault-1",
		Path:      "segredo-beta.md",
		Content:   "conteudo confidencial do cliente beta",
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, repo.Save(ctxA, noteA))
	require.NoError(t, repo.Save(ctxB, noteB))

	// Tenant A lista apenas suas proprias notas
	notesA, err := repo.ListNotes(ctxA)
	require.NoError(t, err)
	require.Len(t, notesA, 1)
	assert.Equal(t, "id-a", notesA[0].ID)

	// Tenant B lista apenas suas proprias notas
	notesB, err := repo.ListNotes(ctxB)
	require.NoError(t, err)
	require.Len(t, notesB, 1)
	assert.Equal(t, "id-b", notesB[0].ID)

	// Tenant B tenta buscar nota A por ID e recebe NotFound
	_, err = repo.GetNoteByID(ctxB, "id-a")
	require.ErrorIs(t, err, notes.ErrNotFound)

	// Busca por conteudo nao vaza entre tenants
	buscaA, err := repo.SearchNotes(ctxA, "confidencial", 0, 10)
	require.NoError(t, err)
	require.Len(t, buscaA, 1)
	assert.Equal(t, "id-a", buscaA[0].ID)

	buscaB, err := repo.SearchNotes(ctxB, "confidencial", 0, 10)
	require.NoError(t, err)
	require.Len(t, buscaB, 1)
	assert.Equal(t, "id-b", buscaB[0].ID)
}

func TestTenantIsolation_MesmoPathEmTenantsDiferentes_Permitido(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)

	ctxA := tenant.WithContext(context.Background(), tenant.Context{TenantID: "tenant-empresa-1"})
	ctxB := tenant.WithContext(context.Background(), tenant.Context{TenantID: "tenant-empresa-2"})

	now := time.Now().UTC().Truncate(time.Millisecond)
	noteA := notes.Note{
		ID:        "id-1",
		TenantID:  "tenant-empresa-1",
		VaultID:   "default",
		Path:      "welcome.md",
		Content:   "bem vindo empresa 1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	noteB := notes.Note{
		ID:        "id-2",
		TenantID:  "tenant-empresa-2",
		VaultID:   "default",
		Path:      "welcome.md",
		Content:   "bem vindo empresa 2",
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, repo.Save(ctxA, noteA))
	require.NoError(t, repo.Save(ctxB, noteB))

	// Criar nota duplicada no mesmo tenant ainda falha
	noteADuplicada := notes.Note{
		ID:        "id-3",
		TenantID:  "tenant-empresa-1",
		VaultID:   "default",
		Path:      "welcome.md",
		Content:   "outra",
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.ErrorIs(t, repo.Save(ctxA, noteADuplicada), notes.ErrDuplicatePath)
}

func TestTenantIsolation_DeleteOuUpdateEmOutroTenant_RetornaNotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db)

	ctxA := tenant.WithContext(context.Background(), tenant.Context{TenantID: "tenant-1"})
	ctxB := tenant.WithContext(context.Background(), tenant.Context{TenantID: "tenant-2"})

	now := time.Now().UTC().Truncate(time.Millisecond)
	noteA := notes.Note{
		ID:        "nota-alvo",
		TenantID:  "tenant-1",
		VaultID:   "default",
		Path:      "alvo.md",
		Content:   "conteudo",
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, repo.Save(ctxA, noteA))

	// Tenant 2 tenta atualizar a nota do Tenant 1
	_, err := repo.Update(ctxB, "nota-alvo", "hack.md", "hacked", now.Add(time.Second), now, true)
	require.ErrorIs(t, err, notes.ErrNotFound)

	// Tenant 2 tenta deletar a nota do Tenant 1
	err = repo.Delete(ctxB, "nota-alvo")
	require.ErrorIs(t, err, notes.ErrNotFound)
}
