package embedding_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
)

// palavras marca cada palavra de "a b c d e" como um token.
var palavras = [][2]int{{0, 1}, {2, 3}, {4, 5}, {6, 7}, {8, 9}}

func TestChunks_CortaOTextoNaFronteiraDosTokens(t *testing.T) {
	pedacos := embedding.Chunks("a b c d e", palavras, 2)

	assert.Equal(t, []string{"a b", "c d", "e"}, pedacos)
}

func TestChunks_FicaComOsOitoPrimeirosPedacos(t *testing.T) {
	spans := make([][2]int, 0, 20)
	for i := range 20 {
		spans = append(spans, [2]int{i, i + 1})
	}

	pedacos := embedding.Chunks("abcdefghijklmnopqrst", spans, 1)

	assert.Equal(t, []string{"a", "b", "c", "d", "e", "f", "g", "h"}, pedacos)
}

func TestChunks_SemToken_NaoDaPedaco(t *testing.T) {
	assert.Empty(t, embedding.Chunks("   ", nil, 512))
}

func TestCharSpans_UmTokenACadaQuatroLetras(t *testing.T) {
	spans := embedding.CharSpans("ação boa!")

	assert.Equal(t, [][2]int{{0, 6}, {6, 10}, {10, 11}}, spans, "conta letras, não bytes: ç e ã têm dois bytes")
}
