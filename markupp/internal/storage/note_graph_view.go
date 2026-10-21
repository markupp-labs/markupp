package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// VaultGraph devolve o grafo do cofre vaultID recortado por f, ou
// notes.ErrVaultForbidden quando o ator não enxerga o cofre.
func (g *PostgresNoteGraph) VaultGraph(ctx context.Context, vaultID string, f notes.GraphFilter) (notes.Graph, error) {
	var out notes.Graph
	err := g.inVault(ctx, vaultID, func(q *gen.Queries) error {
		nodes, err := listGraphNotes(ctx, q, vaultID, nil, f)
		if err != nil {
			return err
		}
		out, err = assembleGraph(ctx, q, vaultID, nodes, nil, nil, f)
		return err
	})
	return out, err
}

// LocalGraph devolve as notas a até depth saltos da nota noteID, nos dois sentidos,
// com as ligações entre elas e os títulos não resolvidos que as de menos de depth
// saltos citam. O filtro f vale também no caminho; a nota lida sempre fica. Devolve
// notes.ErrNotFound quando o ator não enxerga a nota.
func (g *PostgresNoteGraph) LocalGraph(ctx context.Context, noteID string, depth int, f notes.GraphFilter) (notes.Graph, error) {
	var out notes.Graph
	err := g.withNote(ctx, noteID, func(q *gen.Queries, note gen.Note) error {
		hood, err := exploreNeighborhood(ctx, q, note.VaultID, noteID, depth, f)
		if err != nil {
			return err
		}
		out, err = assembleGraph(ctx, q, note.VaultID, hood.nodes, hood.ids(), hood.inner, f)
		return err
	})
	return out, err
}

// VaultGraphVersion devolve a versão do grafo do cofre vaultID, zero antes do
// primeiro segundo estágio, ou notes.ErrVaultForbidden quando o ator não enxerga o
// cofre.
func (g *PostgresNoteGraph) VaultGraphVersion(ctx context.Context, vaultID string) (int64, error) {
	var version int64
	err := g.inVault(ctx, vaultID, func(q *gen.Queries) error {
		var err error
		version, err = q.GetGraphVersion(ctx, vaultID)
		return err
	})
	return version, err
}

// NoteGraphVersion devolve a versão do grafo do cofre da nota noteID, ou
// notes.ErrNotFound quando o ator não enxerga a nota.
func (g *PostgresNoteGraph) NoteGraphVersion(ctx context.Context, noteID string) (int64, error) {
	var version int64
	err := g.withNote(ctx, noteID, func(q *gen.Queries, note gen.Note) error {
		var err error
		version, err = q.GetGraphVersion(ctx, note.VaultID)
		return err
	})
	return version, err
}

// assembleGraph lê as ligações que tocam nas notas edgeIDs e as não resolvidas que
// as notas citingIDs fazem, nil para o cofre inteiro, e monta o grafo sobre nodes.
func assembleGraph(ctx context.Context, q *gen.Queries, vaultID string, nodes []notes.GraphNote, edgeIDs, citingIDs []string, f notes.GraphFilter) (notes.Graph, error) {
	edges, err := listGraphEdges(ctx, q, vaultID, edgeIDs, f)
	if err != nil {
		return notes.Graph{}, err
	}
	citations, err := listGraphCitations(ctx, q, vaultID, citingIDs, f)
	if err != nil {
		return notes.Graph{}, err
	}
	return notes.BuildGraph(nodes, edges, citations), nil
}

// listGraphNotes lê as notas noteIDs do cofre, nil para todas, que passam por f.
func listGraphNotes(ctx context.Context, q *gen.Queries, vaultID string, noteIDs []string, f notes.GraphFilter) ([]notes.GraphNote, error) {
	p := gen.ListGraphNotesParams{VaultID: vaultID, NoteIds: noteIDs, HideOrphans: f.HideOrphans}
	if f.Community != nil {
		p.CommunityID = pgtype.Int8{Int64: *f.Community, Valid: true}
	}
	rows, err := q.ListGraphNotes(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("ler as notas do grafo do cofre %q: %w", vaultID, err)
	}
	return convertRows(rows, toGraphNote), nil
}

func listGraphEdges(ctx context.Context, q *gen.Queries, vaultID string, noteIDs []string, f notes.GraphFilter) ([]notes.GraphEdge, error) {
	rows, err := q.ListGraphEdges(ctx, gen.ListGraphEdgesParams{VaultID: vaultID, NoteIds: noteIDs, HideInferred: f.HideInferred})
	if err != nil {
		return nil, fmt.Errorf("ler as ligacoes do grafo do cofre %q: %w", vaultID, err)
	}
	return convertRows(rows, toGraphEdge), nil
}

// listGraphCitations lê as ligações não resolvidas, nenhuma quando f as esconde.
func listGraphCitations(ctx context.Context, q *gen.Queries, vaultID string, noteIDs []string, f notes.GraphFilter) ([]notes.GraphCitation, error) {
	if f.HideUnresolved {
		return nil, nil
	}
	rows, err := q.ListGraphCitations(ctx, gen.ListGraphCitationsParams{VaultID: vaultID, NoteIds: noteIDs})
	if err != nil {
		return nil, fmt.Errorf("ler as ligacoes nao resolvidas do grafo do cofre %q: %w", vaultID, err)
	}
	return convertRows(rows, toGraphCitation), nil
}

func toGraphNote(r gen.ListGraphNotesRow) notes.GraphNote {
	rank := toNoteRank(gen.GetNoteRankRow{Pagerank: r.Pagerank, InDegree: r.InDegree, OutDegree: r.OutDegree, CommunityID: r.CommunityID})
	return notes.GraphNote{NoteSummary: notes.Summarize(r.ID, r.Path), Rank: rank}
}

func toGraphEdge(r gen.ListGraphEdgesRow) notes.GraphEdge {
	return notes.GraphEdge{SourceID: r.SourceID, TargetID: r.TargetID, Type: r.Kind, Predicate: optionalText(r.Predicate)}
}

func toGraphCitation(r gen.ListGraphCitationsRow) notes.GraphCitation {
	return notes.GraphCitation{NoteID: r.NoteID, TitleKey: r.TitleKey, CitedTitle: r.CitedTitle, Type: r.Kind, Predicate: optionalText(r.Predicate)}
}

func optionalText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}
