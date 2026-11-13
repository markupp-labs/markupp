package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresPublicNotes lê as notas públicas sem conta (ADR-0048), no papel markupp_reader:
// o RLS só deixa ver as notas públicas, e cada leitura recorta pela publicação da nota
// pedida. O id interno da nota fica aqui; para fora só vai o id público.
type PostgresPublicNotes struct {
	pool         *pgxpool.Pool
	searchConfig string
}

// NewPostgresPublicNotes monta a leitura pública sobre um pool já aberto; a busca usa a
// configuração de texto searchConfig, a mesma do indexador.
func NewPostgresPublicNotes(pool *pgxpool.Pool, searchConfig string) *PostgresPublicNotes {
	return &PostgresPublicNotes{pool: pool, searchConfig: searchConfig}
}

// PublicNote devolve a nota publicID com a publicação e as ligações dela.
func (r *PostgresPublicNotes) PublicNote(ctx context.Context, publicID string) (publication.PublicNote, error) {
	var out publication.PublicNote
	err := r.inScope(ctx, publicID, func(q *gen.Queries, s publication.Scope) error {
		row, err := q.GetNoteByID(ctx, s.NoteID)
		if err != nil {
			return fmt.Errorf("read public note %q: %w", publicID, err)
		}
		links, err := q.ListPublicNoteLinks(ctx, s.NoteID)
		out = toPublicNote(publicID, s, row, links)
		return err
	})
	return out, err
}

func toPublicNote(publicID string, s publication.Scope, row gen.Note, links []gen.ListPublicNoteLinksRow) publication.PublicNote {
	title := indexing.Title(row.Path)
	scope := publication.PublicScope{Kind: s.Kind, Title: publication.ScopeTitle(s, title)}
	if s.Kind == publication.KindNote {
		scope.RootPublicID = &publicID
	}
	return publication.PublicNote{
		PublicID: publicID, Title: title, Content: row.Content, UpdatedAt: row.UpdatedAt,
		Indexable: s.Indexable, Publication: scope, Links: convertRows(links, toPublicLink),
	}
}

func toPublicLink(r gen.ListPublicNoteLinksRow) publication.PublicLink {
	link := publication.PublicLink{CitedTitle: r.CitedTitle, Kind: r.Kind}
	if r.PublicID.Valid {
		title := indexing.Title(r.TargetPath.String)
		link.PublicID, link.TargetTitle = &r.PublicID.String, &title
	}
	return link
}

// PublicTree devolve a página das notas da publicação de publicID e o total delas.
func (r *PostgresPublicNotes) PublicTree(ctx context.Context, publicID string, offset, limit int) ([]publication.PublicTreeItem, int, error) {
	var items []publication.PublicTreeItem
	var total int64
	err := r.inScope(ctx, publicID, func(q *gen.Queries, s publication.Scope) error {
		rows, err := q.ListPublicTree(ctx, gen.ListPublicTreeParams{PublicationID: s.PublicationID, PageLimit: toInt32(limit), PageOffset: toInt32(offset)})
		if err != nil {
			return fmt.Errorf("list the publication of %q: %w", publicID, err)
		}
		items = convertRows(rows, func(row gen.ListPublicTreeRow) publication.PublicTreeItem {
			return publication.PublicTreeItem{PublicID: row.PublicID, Title: indexing.Title(row.Path), Path: publication.RelativePath(s, row.Path)}
		})
		total, err = q.CountPublicTree(ctx, s.PublicationID)
		return err
	})
	return items, int(total), err
}

// SearchPublic busca query nas notas da publicação de publicID, da mais relevante para a
// menos, e devolve também o total de resultados.
func (r *PostgresPublicNotes) SearchPublic(ctx context.Context, publicID, query string, offset, limit int) ([]publication.PublicSearchHit, int, error) {
	var hits []publication.PublicSearchHit
	var total int64
	err := r.inScope(ctx, publicID, func(q *gen.Queries, s publication.Scope) error {
		rows, err := q.SearchPublicNotes(ctx, gen.SearchPublicNotesParams{
			SearchConfig: r.searchConfig, Query: query, PublicationID: s.PublicationID, PageLimit: toInt32(limit), PageOffset: toInt32(offset),
		})
		if err != nil {
			return fmt.Errorf("search the publication of %q: %w", publicID, err)
		}
		hits = convertRows(rows, toPublicSearchHit)
		total, err = q.CountPublicSearch(ctx, gen.CountPublicSearchParams{SearchConfig: r.searchConfig, Query: query, PublicationID: s.PublicationID})
		return err
	})
	return hits, int(total), err
}

func toPublicSearchHit(r gen.SearchPublicNotesRow) publication.PublicSearchHit {
	return publication.PublicSearchHit{PublicID: r.PublicID, Title: indexing.Title(r.Path), Snippet: r.Snippet, Rank: r.Rank}
}

// PublicGraphTag devolve o validador do grafo da publicação de publicID, vazio antes do
// primeiro segundo estágio.
func (r *PostgresPublicNotes) PublicGraphTag(ctx context.Context, publicID string) (string, error) {
	var tag string
	err := r.inScope(ctx, publicID, func(q *gen.Queries, s publication.Scope) error {
		version, err := q.GetGraphVersion(ctx, s.VaultID)
		tag = publication.GraphTag(version, s.PublicationID)
		return err
	})
	return tag, err
}

// PublicGraph devolve o grafo das notas da publicação de publicID.
func (r *PostgresPublicNotes) PublicGraph(ctx context.Context, publicID string) (publication.PublicGraph, error) {
	var out publication.PublicGraph
	err := r.inScope(ctx, publicID, func(q *gen.Queries, s publication.Scope) error {
		nodes, err := q.ListPublicGraphNodes(ctx, s.PublicationID)
		if err != nil {
			return fmt.Errorf("list the graph nodes of %q: %w", publicID, err)
		}
		edges, err := q.ListPublicGraphEdges(ctx, s.PublicationID)
		out = publication.BuildPublicGraph(convertRows(nodes, toPublicGraphNode), convertRows(edges, toPublicEdge))
		return err
	})
	return out, err
}

func toPublicGraphNode(r gen.ListPublicGraphNodesRow) publication.PublicGraphNode {
	return publication.PublicGraphNode{PublicID: r.PublicID, Title: indexing.Title(r.Path)}
}

func toPublicEdge(r gen.ListPublicGraphEdgesRow) publication.PublicEdge {
	return publication.PublicEdge{SourcePublicID: r.SourcePublicID, TargetPublicID: r.TargetPublicID, Type: r.Kind}
}

// inScope roda read na transação pública com a publicação da nota publicID. Id fora do
// formato, inexistente ou de nota que deixou de ser pública dão o mesmo erro.
func (r *PostgresPublicNotes) inScope(ctx context.Context, publicID string, read func(q *gen.Queries, s publication.Scope) error) error {
	if !publication.ValidPublicID(publicID) {
		return fmt.Errorf("%w: %q", publication.ErrPublicNoteNotFound, publicID)
	}
	return inPublicTx(ctx, r.pool, func(q *gen.Queries) error {
		row, err := q.GetPublicScope(ctx, publicID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: %q", publication.ErrPublicNoteNotFound, publicID)
		}
		if err != nil {
			return fmt.Errorf("read the publication of %q: %w", publicID, err)
		}
		return read(q, publication.Scope(row))
	})
}
