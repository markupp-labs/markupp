package storage

import (
	"context"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// neighborhood é a vizinhança de uma nota em largura: as notas visitadas, na ordem
// em que foram alcançadas, seen, as já consideradas, visitadas ou recusadas pelo
// filtro, e inner, as que já foram expandidas.
type neighborhood struct {
	nodes []notes.GraphNote
	seen  map[string]bool
	inner []string
}

// exploreNeighborhood visita, nível a nível, as notas a até depth saltos da nota
// centerID que passam por f, com uma leitura das ligações e uma das notas por nível.
// A nota central entra sem filtro.
func exploreNeighborhood(ctx context.Context, q *gen.Queries, vaultID, centerID string, depth int, f notes.GraphFilter) (*neighborhood, error) {
	center, err := listGraphNotes(ctx, q, vaultID, []string{centerID}, notes.GraphFilter{})
	if err != nil {
		return nil, err
	}
	hood := &neighborhood{seen: map[string]bool{}}
	frontier := hood.add(center)
	for level := 0; level < depth && len(frontier) > 0; level++ {
		hood.inner = append(hood.inner, frontier...)
		if frontier, err = hood.expand(ctx, q, vaultID, frontier, f); err != nil {
			return nil, err
		}
	}
	return hood, nil
}

// expand visita as notas ainda não vistas ligadas às notas frontier e devolve os ids
// delas, a próxima fronteira.
func (h *neighborhood) expand(ctx context.Context, q *gen.Queries, vaultID string, frontier []string, f notes.GraphFilter) ([]string, error) {
	edges, err := listGraphEdges(ctx, q, vaultID, frontier, f)
	if err != nil {
		return nil, err
	}
	reached, err := listGraphNotes(ctx, q, vaultID, h.unseen(edges), f)
	if err != nil {
		return nil, err
	}
	return h.add(reached), nil
}

// unseen devolve as pontas das ligações ainda não consideradas, sem repetir e nunca
// nil, porque nil leria o cofre inteiro. A ponta que o filtro recusar não volta a ser
// lida no próximo nível.
func (h *neighborhood) unseen(edges []notes.GraphEdge) []string {
	ids := []string{}
	for _, e := range edges {
		ids = h.claim(ids, e.SourceID)
		ids = h.claim(ids, e.TargetID)
	}
	return ids
}

// claim acrescenta id a ids e o marca como considerado, se ainda não estava.
func (h *neighborhood) claim(ids []string, id string) []string {
	if h.seen[id] {
		return ids
	}
	h.seen[id] = true
	return append(ids, id)
}

// add visita as notas e devolve os ids delas.
func (h *neighborhood) add(reached []notes.GraphNote) []string {
	ids := make([]string, 0, len(reached))
	for _, n := range reached {
		h.seen[n.ID] = true
		h.nodes = append(h.nodes, n)
		ids = append(ids, n.ID)
	}
	return ids
}

// ids devolve os ids das notas visitadas, nunca nil.
func (h *neighborhood) ids() []string {
	ids := make([]string, 0, len(h.nodes))
	for _, n := range h.nodes {
		ids = append(ids, n.ID)
	}
	return ids
}
