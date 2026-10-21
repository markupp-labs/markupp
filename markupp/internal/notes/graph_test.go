package notes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

func TestSummarize_TituloVemDoUltimoSegmentoDoCaminho(t *testing.T) {
	assert.Equal(t, notes.NoteSummary{ID: "n1", Title: "Reunião", Path: "pasta/Reunião.md"}, notes.Summarize("n1", "pasta/Reunião.md"))
	assert.Equal(t, "foto.png", notes.Summarize("n2", "anexos/foto.png").Title, "sem .md, o nome inteiro")
}
