package embedding

import (
	"context"
	"fmt"
)

// Provider transforma texto em vetor. É a fronteira do projeto com o modelo, local
// ou remoto.
type Provider interface {
	// Name identifica o provedor, o modelo e o arquivo: outro nome refaz os vetores.
	Name() string
	// TokenSpans devolve o início e o fim, em bytes, de cada token de text, sem os
	// tokens especiais.
	TokenSpans(text string) [][2]int
	// Embed devolve um vetor normalizado por texto, na ordem de texts.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Indexer refaz o vetor das notas pendentes, um lote de um cofre por chamada.
type Indexer struct {
	queue       Queue
	provider    Provider
	chunkTokens int
	batchSize   int
}

// NewIndexer monta o indexador com pedaços de chunkTokens tokens e lotes de até
// batchSize notas.
func NewIndexer(queue Queue, provider Provider, chunkTokens, batchSize int) *Indexer {
	return &Indexer{queue: queue, provider: provider, chunkTokens: chunkTokens, batchSize: batchSize}
}

// IndexNext refaz os vetores do próximo lote e devolve false quando a fila está vazia.
func (x *Indexer) IndexNext(ctx context.Context) (bool, error) {
	batch, found, err := x.queue.NextBatch(ctx, x.batchSize)
	if !found || err != nil {
		return found, err
	}
	vectors := make([]NoteVector, 0, len(batch.Notes))
	for _, note := range batch.Notes {
		vector, err := x.noteVector(ctx, note.Content)
		if err != nil {
			return true, fmt.Errorf("embed note %q of vault %q: %w", note.NoteID, batch.VaultID, err)
		}
		vectors = append(vectors, NoteVector{Note: note, Vector: vector})
	}
	return true, x.queue.SaveVectors(ctx, batch, vectors)
}

// noteVector é a média normalizada dos vetores dos pedaços de text, ou nil sem texto.
func (x *Indexer) noteVector(ctx context.Context, text string) ([]float32, error) {
	chunks := Chunks(text, x.provider.TokenSpans(text), x.chunkTokens)
	if len(chunks) == 0 {
		return nil, nil
	}
	vectors, err := x.provider.Embed(ctx, chunks)
	if err != nil {
		return nil, err
	}
	return MeanNormalized(vectors)
}

// ModelRegistry registra o modelo em uso e diz se ele trocou.
type ModelRegistry func(ctx context.Context, name string, dimension int) (bool, error)

// Activate descobre a dimensão do provedor por um texto de prova e registra o modelo,
// que, quando trocou, refaz todos os vetores.
func Activate(ctx context.Context, provider Provider, use ModelRegistry) (bool, error) {
	probe, err := provider.Embed(ctx, []string{"markupp"})
	if err != nil {
		return false, fmt.Errorf("probe embedding model %q: %w", provider.Name(), err)
	}
	if len(probe) != 1 || len(probe[0]) == 0 {
		return false, fmt.Errorf("probe embedding model %q returned %d vectors, expected one nonempty vector", provider.Name(), len(probe))
	}
	return use(ctx, provider.Name(), len(probe[0]))
}
