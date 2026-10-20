package storagetest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

// NoteSaver é o Save do repositório de notas. storagetest não importa storage
// porque os testes internos de storage importam storagetest.
type NoteSaver interface {
	Save(ctx context.Context, note notes.Note) (notes.Note, error)
}

// SaveNote grava note por repo no contexto ctx, falha o teste se der erro e
// devolve a nota gravada.
func SaveNote(ctx context.Context, t *testing.T, repo NoteSaver, note notes.Note) notes.Note {
	t.Helper()
	saved, err := repo.Save(ctx, note)
	require.NoError(t, err, "salvar nota %q no cofre %q", note.ID, note.VaultID)
	return saved
}
