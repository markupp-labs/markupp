package notes

import (
	"context"
	"fmt"
)

// NoteRank é a organização derivada da nota que o segundo estágio do indexador grava:
// o grau (notas distintas que ligam a ela mais as que ela liga, inferidas incluídas),
// o PageRank e a comunidade. Tudo nil antes do segundo estágio; Community nil também
// na nota que não tem comunidade.
type NoteRank struct {
	Degree    *int
	PageRank  *float64
	Community *int64
}

// Ordens da lista das notas do cofre; a vazia é por caminho.
const (
	SortByPath     = ""
	SortByPageRank = "pagerank"
	SortByDegree   = "degree"
	SortByUpdated  = "updated"
)

// VaultNotesQuery recorta e ordena a lista das notas de um cofre. Orphan fica com as
// notas de grau zero, ou com as de grau positivo quando falso, e Community com as da
// comunidade; ambos ignoram a nota sem organização derivada. Sort ordena, decrescente
// com Descending, e a ordem por caminho é sempre crescente. Nil e vazio não filtram.
type VaultNotesQuery struct {
	Orphan     *bool
	Community  *int64
	Sort       string
	Descending bool
	Offset     int
	Limit      int
}

// ListVaultNotes devolve a página das notas do cofre vaultID que q pede.
func (s *Service) ListVaultNotes(ctx context.Context, vaultID string, q VaultNotesQuery) ([]Note, error) {
	list, err := s.repo.ListVaultNotes(ctx, vaultID, q)
	if err != nil {
		return nil, fmt.Errorf("listar as notas do cofre %q: %w", vaultID, err)
	}
	return list, nil
}
