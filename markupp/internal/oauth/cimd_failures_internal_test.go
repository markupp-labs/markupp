package oauth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var instanteDaFalha = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

func TestCIMDFailureMemory_FalhaLembrada_VaiAteOFimDoPrazo(t *testing.T) {
	memory := newCIMDFailureMemory()
	fim := instanteDaFalha.Add(5 * time.Minute)
	memory.remember("https://a.exemplo.dev/c.json", fim, instanteDaFalha)

	until, ok := memory.failedUntil("https://a.exemplo.dev/c.json", fim.Add(-time.Second))
	assert.True(t, ok)
	assert.Equal(t, fim, until)
	_, ok = memory.failedUntil("https://a.exemplo.dev/c.json", fim)
	assert.False(t, ok, "no fim do prazo a falha já venceu")
	_, ok = memory.failedUntil("https://b.exemplo.dev/c.json", instanteDaFalha)
	assert.False(t, ok, "outro client_id não é afetado")
}

func TestCIMDFailureMemory_NovaFalha_DescartaAsVencidas(t *testing.T) {
	memory := newCIMDFailureMemory()
	memory.remember("https://a.exemplo.dev/c.json", instanteDaFalha.Add(time.Minute), instanteDaFalha)

	depois := instanteDaFalha.Add(2 * time.Minute)
	memory.remember("https://b.exemplo.dev/c.json", depois.Add(time.Minute), depois)

	assert.Equal(t, map[string]time.Time{"https://b.exemplo.dev/c.json": depois.Add(time.Minute)}, memory.until)
}
