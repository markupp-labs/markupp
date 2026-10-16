// Package indexing calcula o dado derivado de um cofre a partir das notas que
// mudaram nele (ADR-0033). Não guarda estado e não sabe quem o invocou: o Job do
// cluster e o laço do compose são entrypoints finos sobre o mesmo pacote.
package indexing

import (
	"context"
	"log/slog"
	"path"
	"strings"
)

// NoteChange é uma nota que mudou desde a última indexação do cofre.
type NoteChange struct {
	NoteID  string
	Path    string
	Content string
}

// Changeset são as notas de um cofre que mudaram desde a última indexação dele.
type Changeset struct {
	TenantID string
	VaultID  string
	Notes    []NoteChange
}

// SearchDocument é o texto que o banco converte em tsvector para a busca léxica: o
// título com peso A e o corpo com peso D.
type SearchDocument struct {
	NoteID string
	Title  string
	Text   string
}

// IndexFunc transforma um lote de mudanças nas linhas derivadas dele.
type IndexFunc func(Changeset) []SearchDocument

// VaultIndexer reserva o próximo cofre pendente, entrega o lote dele a index e grava
// o resultado. Devolve false quando não há cofre pendente livre.
type VaultIndexer interface {
	IndexNextVault(ctx context.Context, index IndexFunc) (bool, error)
}

// Index devolve um documento de busca por nota mudada, com o título e o conteúdo dela.
func Index(cs Changeset) []SearchDocument {
	docs := make([]SearchDocument, 0, len(cs.Notes))
	for _, note := range cs.Notes {
		docs = append(docs, SearchDocument{NoteID: note.NoteID, Title: Title(note.Path), Text: note.Content})
	}
	return docs
}

// Title é o título da nota: o último segmento do caminho, sem a extensão .md em
// qualquer caixa. Outra extensão fica, porque faz parte do nome.
func Title(notePath string) string {
	base := path.Base(notePath)
	if !strings.EqualFold(path.Ext(base), ".md") {
		return base
	}
	return base[:len(base)-len(".md")]
}

// RunLoop indexa os cofres pendentes até não sobrar nenhum, chama wait e recomeça.
// Erro de indexação vai para o log e também espera wait, para o laço não martelar o
// banco. Termina só quando wait devolve erro.
func RunLoop(ctx context.Context, indexer VaultIndexer, wait func(context.Context) error, logger *slog.Logger) error {
	for {
		found, err := indexer.IndexNextVault(ctx, Index)
		if err != nil {
			logger.Error("indexar proximo cofre", "err", err)
		}
		if found && err == nil {
			continue
		}
		if err := wait(ctx); err != nil {
			return err
		}
	}
}
