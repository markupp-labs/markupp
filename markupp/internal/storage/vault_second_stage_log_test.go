package storage_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// registrosDoTeto devolve os registros JSON de saida que avisam um cálculo do
// segundo estágio parado no teto.
func registrosDoTeto(t *testing.T, saida []byte) []map[string]any {
	t.Helper()
	var registros []map[string]any
	for _, linha := range bytes.Split(bytes.TrimSpace(saida), []byte("\n")) {
		var registro map[string]any
		require.NoError(t, json.Unmarshal(linha, &registro), "linha de log fora do JSON: %s", linha)
		if registro["msg"] == "calculo do segundo estagio parou no teto" {
			registros = append(registros, registro)
		}
	}
	return registros
}

func TestSegundoEstagio_PageRankNoTeto_RegistraOCofreEAsIteracoes(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for id, conteudo := range map[string]string{"a": "[[b]]", "b": "[[c]]", "c": "[[a]]", "d": "[[a]]"} {
		storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho(id, id+".md", conteudo))
	}
	var saida bytes.Buffer
	idx, err := storage.NewPostgresVaultIndexer(context.Background(), db, "simple", slog.New(slog.NewJSONHandler(&saida, nil)))
	require.NoError(t, err)

	for found := true; found; {
		found, _ = indexaProximo(t, idx)
	}

	registros := registrosDoTeto(t, saida.Bytes())
	require.Len(t, registros, 1, "o ciclo a-b-c com d citando a leva o PageRank ao teto: %s", saida.String())
	assert.Equal(t, "pagerank", registros[0]["algoritmo"])
	assert.Equal(t, cofreDeTeste, registros[0]["cofre"])
	assert.InDelta(t, 100, registros[0]["iteracoes"], 0)
}
