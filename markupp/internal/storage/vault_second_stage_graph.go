package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/vaultgraph"
)

// refreshVaultGraph refaz o que depende do grafo inteiro do cofre. As ligações
// inferidas vêm primeiro, porque entram no grafo da centralidade e das comunidades.
// O PageRank e o Louvain que param no teto são avisados em logger.
func refreshVaultGraph(ctx context.Context, q *gen.Queries, vaultID, searchConfig string, logger *slog.Logger) error {
	if err := q.DeleteInferredLinks(ctx, vaultID); err != nil {
		return fmt.Errorf("apagar as ligacoes inferidas do cofre %q: %w", vaultID, err)
	}
	noteIDs, edges, err := loadVaultEdges(ctx, q, vaultID)
	if err != nil {
		return err
	}
	inferred, err := inferLinks(ctx, q, vaultID, searchConfig, vaultgraph.New(noteIDs, edges))
	if err != nil {
		return err
	}
	g := vaultgraph.New(noteIDs, append(edges, inferred...))
	if err := saveCentrality(ctx, q, vaultID, noteIDs, g, logger); err != nil {
		return err
	}
	return saveCommunities(ctx, q, vaultID, g, logger)
}

// loadVaultEdges lê as notas e as ligações resolvidas do cofre.
func loadVaultEdges(ctx context.Context, q *gen.Queries, vaultID string) ([]string, []vaultgraph.Edge, error) {
	noteIDs, err := q.ListVaultNoteIDs(ctx, vaultID)
	if err != nil {
		return nil, nil, fmt.Errorf("listar as notas do cofre %q: %w", vaultID, err)
	}
	rows, err := q.ListVaultEdges(ctx, vaultID)
	if err != nil {
		return nil, nil, fmt.Errorf("listar as ligacoes resolvidas do cofre %q: %w", vaultID, err)
	}
	edges := make([]vaultgraph.Edge, len(rows))
	for i, r := range rows {
		edges[i] = vaultgraph.Edge{Source: r.SourceID, Target: r.TargetID}
	}
	return noteIDs, edges, nil
}
