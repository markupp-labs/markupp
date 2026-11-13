package embedding_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
)

// fakeEmbeddingQueue entrega os lotes de batches em ordem e guarda o que foi gravado.
type fakeEmbeddingQueue struct {
	batches []embedding.PendingBatch
	limits  []int
	saved   []embedding.NoteVector
}

func (f *fakeEmbeddingQueue) NextBatch(_ context.Context, limit int) (embedding.PendingBatch, bool, error) {
	f.limits = append(f.limits, limit)
	if len(f.batches) == 0 {
		return embedding.PendingBatch{}, false, nil
	}
	next := f.batches[0]
	f.batches = f.batches[1:]
	return next, true, nil
}

func (f *fakeEmbeddingQueue) SaveVectors(_ context.Context, _ embedding.PendingBatch, vectors []embedding.NoteVector) error {
	f.saved = append(f.saved, vectors...)
	return nil
}

// fakeEmbeddingProvider trata cada caractere como token e dá ao pedaço que começa
// com x o vetor [1, 0] e aos outros [0, 1], para a média ser previsível.
type fakeEmbeddingProvider struct {
	calls [][]string
	err   error
}

func (f *fakeEmbeddingProvider) Name() string { return "fake" }

func (f *fakeEmbeddingProvider) TokenSpans(text string) [][2]int {
	spans := make([][2]int, 0, len(text))
	for i := range len(text) {
		spans = append(spans, [2]int{i, i + 1})
	}
	return spans
}

func (f *fakeEmbeddingProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	f.calls = append(f.calls, texts)
	if f.err != nil {
		return nil, f.err
	}
	vectors := make([][]float32, 0, len(texts))
	for _, text := range texts {
		if strings.HasPrefix(text, "x") {
			vectors = append(vectors, []float32{1, 0})
			continue
		}
		vectors = append(vectors, []float32{0, 1})
	}
	return vectors, nil
}

func loteDeTeste(notas ...embedding.PendingNote) *fakeEmbeddingQueue {
	return &fakeEmbeddingQueue{batches: []embedding.PendingBatch{{TenantID: "t", VaultID: "v", Notes: notas}}}
}

func TestIndexNext_GravaAMediaNormalizadaDosPedacosDeCadaNota(t *testing.T) {
	fila := loteDeTeste(embedding.PendingNote{NoteID: "a", Content: "xxyy", Revision: 7})
	provedor := &fakeEmbeddingProvider{}

	found, err := embedding.NewIndexer(fila, provedor, 2, 16).IndexNext(context.Background())

	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, [][]string{{"xx", "yy"}}, provedor.calls, "pedaços de dois tokens")
	require.Len(t, fila.saved, 1)
	assert.Equal(t, int64(7), fila.saved[0].Note.Revision)
	assert.InDeltaSlice(t, []float32{0.70710677, 0.70710677}, fila.saved[0].Vector, 1e-6)
	assert.Equal(t, []int{16}, fila.limits)
}

func TestIndexNext_NotaSemTexto_GravaSemVetorESemChamarOModelo(t *testing.T) {
	fila := loteDeTeste(embedding.PendingNote{NoteID: "vazia", Content: ""})
	provedor := &fakeEmbeddingProvider{}

	_, err := embedding.NewIndexer(fila, provedor, 512, 16).IndexNext(context.Background())

	require.NoError(t, err)
	assert.Empty(t, provedor.calls)
	require.Len(t, fila.saved, 1)
	assert.Nil(t, fila.saved[0].Vector)
}

func TestIndexNext_FilaVazia_DevolveFalse(t *testing.T) {
	found, err := embedding.NewIndexer(&fakeEmbeddingQueue{}, &fakeEmbeddingProvider{}, 512, 16).IndexNext(context.Background())

	require.NoError(t, err)
	assert.False(t, found)
}

func TestIndexNext_ErroDoModelo_NaoGravaNada(t *testing.T) {
	fila := loteDeTeste(embedding.PendingNote{NoteID: "a", Content: "texto"})
	provedor := &fakeEmbeddingProvider{err: errors.New("modelo fora do ar")}

	_, err := embedding.NewIndexer(fila, provedor, 512, 16).IndexNext(context.Background())

	require.ErrorContains(t, err, "modelo fora do ar")
	assert.Contains(t, err.Error(), `"a"`)
	assert.Empty(t, fila.saved)
}

func TestActivate_RegistraONomeEADimensaoQueOModeloDevolve(t *testing.T) {
	var nome string
	var dimensao int
	usa := func(_ context.Context, name string, dimension int) (bool, error) {
		nome, dimensao = name, dimension
		return true, nil
	}

	trocou, err := embedding.Activate(context.Background(), &fakeEmbeddingProvider{}, usa)

	require.NoError(t, err)
	assert.True(t, trocou)
	assert.Equal(t, "fake", nome)
	assert.Equal(t, 2, dimensao)
}
