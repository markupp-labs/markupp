// Package embedding é o indexador de embedding: transforma o texto de cada nota num
// vetor, a média dos vetores dos pedaços dela (ADR-0056), que o segundo estágio do
// indexador de texto usa para a ligação por sentido (ADR-0053). Não guarda estado:
// o laço do compose e qualquer outro entrypoint são finos sobre ele, como em indexing.
package embedding

import "context"

// PendingNote é uma nota com o vetor por refazer: o texto, o hash dele e a revisão da
// fila, lidos juntos.
type PendingNote struct {
	NoteID      string
	Content     string
	ContentHash []byte
	Revision    int64
}

// PendingBatch são notas pendentes de um mesmo cofre.
type PendingBatch struct {
	TenantID string
	VaultID  string
	Notes    []PendingNote
}

// NoteVector é o vetor de uma nota pendente, nil quando a nota não tem texto.
type NoteVector struct {
	Note   PendingNote
	Vector []float32
}

// Queue é a fila do indexador de embedding.
type Queue interface {
	// NextBatch lê até limit notas pendentes de um cofre, e false quando não há nenhuma.
	NextBatch(ctx context.Context, limit int) (PendingBatch, bool, error)
	// SaveVectors grava os vetores do lote e tira da fila as revisões lidas. A nota que
	// mudou depois da leitura fica sem vetor e na fila.
	SaveVectors(ctx context.Context, batch PendingBatch, vectors []NoteVector) error
}
