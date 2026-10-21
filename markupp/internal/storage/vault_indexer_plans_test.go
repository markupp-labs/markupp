package storage_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

func TestVaultIndexer_TermosDoLote_PlanoNaoPercorreAsNotasDoCofre(t *testing.T) {
	db := setupNotesDB(t)
	storagetest.SaveNote(ctxDeTeste, t, storage.NewPostgresNotesRepository(db, "simple"), notaComCaminho("n1", "n1.md", "um dois"))
	pool := poolDeUmaConexao(t, db)
	indexaProximo(t, novoIndexador(t, pool, "simple"))

	plano := strings.Join(planoGenerico(t, pool, "SaveNoteTerms"), "\n")

	assert.NotContains(t, plano, "on notes",
		"os termos partem dos documentos do lote; juntar com notes faz o plano percorrer as notas do cofre e sondar o lote a cada uma: %s", plano)
}
