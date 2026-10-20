package storage

import (
	"context"
	"fmt"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/vaultgraph"
)

// saveCentrality troca o PageRank e os graus das notas noteIDs do cofre pelos do
// grafo g.
func saveCentrality(ctx context.Context, q *gen.Queries, vaultID string, noteIDs []string, g *vaultgraph.Graph) error {
	if err := q.DeleteNoteCentrality(ctx, vaultID); err != nil {
		return fmt.Errorf("apagar a centralidade do cofre %q: %w", vaultID, err)
	}
	ranks, _ := g.PageRank()
	params := gen.SaveNoteCentralityParams{NoteIds: noteIDs, Pageranks: make([]float64, len(noteIDs)),
		InDegrees: make([]int32, len(noteIDs)), OutDegrees: make([]int32, len(noteIDs))}
	for i, id := range noteIDs {
		in, out := g.Degree(id)
		params.Pageranks[i], params.InDegrees[i], params.OutDegrees[i] = ranks[id], toInt32(in), toInt32(out)
	}
	if err := q.SaveNoteCentrality(ctx, params); err != nil {
		return fmt.Errorf("gravar a centralidade das %d notas do cofre %q: %w", len(noteIDs), vaultID, err)
	}
	return nil
}

// saveCommunities troca as comunidades do cofre pelas do grafo g. Cada comunidade
// herda o id da anterior que mais se sobrepõe a ela, se o Jaccard for de pelo menos
// 0,5, ou ganha um id novo da sequência.
func saveCommunities(ctx context.Context, q *gen.Queries, vaultID string, g *vaultgraph.Graph) error {
	previous, err := previousCommunities(ctx, q, vaultID)
	if err != nil {
		return err
	}
	communities, _ := g.Communities()
	ids, err := assignCommunityIDs(ctx, q, vaultID, vaultgraph.InheritCommunityIDs(communities, previous))
	if err != nil {
		return err
	}
	if err := q.DeleteNoteCommunities(ctx, vaultID); err != nil {
		return fmt.Errorf("apagar as comunidades do cofre %q: %w", vaultID, err)
	}
	if err := q.SaveNoteCommunities(ctx, communityRows(communities, ids)); err != nil {
		return fmt.Errorf("gravar as %d comunidades do cofre %q: %w", len(communities), vaultID, err)
	}
	return nil
}

func previousCommunities(ctx context.Context, q *gen.Queries, vaultID string) (map[string]int64, error) {
	rows, err := q.ListNoteCommunities(ctx, vaultID)
	if err != nil {
		return nil, fmt.Errorf("listar as comunidades anteriores do cofre %q: %w", vaultID, err)
	}
	previous := make(map[string]int64, len(rows))
	for _, r := range rows {
		previous[r.NoteID] = r.CommunityID
	}
	return previous, nil
}

// assignCommunityIDs troca cada zero de inherited, comunidade sem id herdado, por um
// id novo da sequência.
func assignCommunityIDs(ctx context.Context, q *gen.Queries, vaultID string, inherited []int64) ([]int64, error) {
	missing := 0
	for _, id := range inherited {
		if id == 0 {
			missing++
		}
	}
	if missing == 0 {
		return inherited, nil
	}
	fresh, err := q.NextCommunityIDs(ctx, toInt32(missing))
	if err != nil {
		return nil, fmt.Errorf("reservar %d ids de comunidade para o cofre %q: %w", missing, vaultID, err)
	}
	for i := range inherited {
		if inherited[i] == 0 {
			inherited[i], fresh = fresh[0], fresh[1:]
		}
	}
	return inherited, nil
}

func communityRows(communities [][]string, ids []int64) gen.SaveNoteCommunitiesParams {
	var p gen.SaveNoteCommunitiesParams
	for i, members := range communities {
		for _, id := range members {
			p.NoteIds = append(p.NoteIds, id)
			p.CommunityIds = append(p.CommunityIds, ids[i])
		}
	}
	return p
}
