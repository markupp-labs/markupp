package storage

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/vaultgraph"
)

func TestLogCapped_PageRankNoTeto_RegistraOCofreEAsIteracoes(t *testing.T) {
	var saida bytes.Buffer

	logCapped(slog.New(slog.NewJSONHandler(&saida, nil)), "cofre-x", "pagerank", vaultgraph.Convergence{Iterations: 100, Capped: true})

	var registro map[string]any
	require.NoError(t, json.Unmarshal(saida.Bytes(), &registro), "log fora do JSON: %s", saida.String())
	assert.Equal(t, "calculo do segundo estagio parou no teto", registro["msg"])
	assert.Equal(t, "pagerank", registro["algoritmo"])
	assert.Equal(t, "cofre-x", registro["cofre"])
	assert.InDelta(t, 100, registro["iteracoes"], 0)
}

func TestLogCapped_ConvergenciaAntesDoTetoNaoRegistra(t *testing.T) {
	var saida bytes.Buffer

	logCapped(slog.New(slog.NewJSONHandler(&saida, nil)), "cofre-x", "pagerank", vaultgraph.Convergence{Iterations: 80})

	assert.Empty(t, saida.String())
}
