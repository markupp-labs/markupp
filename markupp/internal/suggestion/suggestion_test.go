package suggestion_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/suggestion"
)

func TestFuse_SomaOInversoDaPosicaoMaisSessenta(t *testing.T) {
	fused := suggestion.Fuse([]string{"a", "b"}, []string{"b", "c"})

	assert.InDelta(t, 1.0/61, fused["a"], 1e-12)
	assert.InDelta(t, 1.0/62+1.0/61, fused["b"], 1e-12, "quem aparece nas duas listas soma as duas")
	assert.InDelta(t, 1.0/62, fused["c"], 1e-12)
	assert.Len(t, fused, 3)
}

func TestFuse_SemListas_DevolveVazio(t *testing.T) {
	assert.Empty(t, suggestion.Fuse())
	assert.Empty(t, suggestion.Fuse(nil, []string{}))
}
