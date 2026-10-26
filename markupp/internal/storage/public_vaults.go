package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresPublicVaults lê os cofres publicados sem conta (ADR-0048), no papel
// markupp_reader: o RLS só deixa ver o que está nos cofres publicados, e as consultas
// reusadas das rotas autenticadas recortam pelo cofre do slug.
type PostgresPublicVaults struct {
	pool         *pgxpool.Pool
	searchConfig string
}

// NewPostgresPublicVaults monta a leitura pública sobre um pool já aberto; a busca usa
// a configuração de texto searchConfig, a mesma do indexador.
func NewPostgresPublicVaults(pool *pgxpool.Pool, searchConfig string) *PostgresPublicVaults {
	return &PostgresPublicVaults{pool: pool, searchConfig: searchConfig}
}

// PublicVault devolve o cofre publicado com slug e a versão do grafo dele, ou
// publication.ErrNotPublished quando nenhum cofre está publicado com esse slug.
func (r *PostgresPublicVaults) PublicVault(ctx context.Context, slug string) (publication.PublicVault, error) {
	var out publication.PublicVault
	err := r.inVault(ctx, slug, func(q *gen.Queries, vault publication.PublicVault) error {
		version, err := q.GetGraphVersion(ctx, vault.VaultID)
		out = vault
		out.GraphVersion = version
		return err
	})
	return out, err
}

// PublicNotes devolve a página das notas do cofre publicado com slug que vq pede, sem
// o conteúdo.
func (r *PostgresPublicVaults) PublicNotes(ctx context.Context, slug string, vq notes.VaultNotesQuery) ([]notes.Note, error) {
	var out []notes.Note
	err := r.inVault(ctx, slug, func(q *gen.Queries, vault publication.PublicVault) error {
		rows, err := listVaultNotes(ctx, q, vaultNotesParams(vault.VaultID, vq))
		out = convertRows(rows, toRankedNote)
		return err
	})
	return out, err
}

// PublicNote devolve a nota noteID do cofre publicado com slug, ou notes.ErrNotFound
// quando ela não existe nele, mesmo que exista em outro cofre publicado.
func (r *PostgresPublicVaults) PublicNote(ctx context.Context, slug, noteID string) (notes.Note, error) {
	var out notes.Note
	err := r.inVault(ctx, slug, func(q *gen.Queries, vault publication.PublicVault) error {
		row, err := q.GetNoteByID(ctx, noteID)
		if err != nil {
			return notFoundOnNoRows(err)
		}
		if row.VaultID != vault.VaultID {
			return fmt.Errorf("%w: nota %q fora do cofre publicado %q", notes.ErrNotFound, noteID, slug)
		}
		out, err = withRank(ctx, q, toNote(row))
		return err
	})
	return out, err
}

// SearchPublicNotes busca no índice léxico do cofre publicado com slug, da nota mais
// relevante para a menos.
func (r *PostgresPublicVaults) SearchPublicNotes(ctx context.Context, slug, query string, offset, limit int) ([]notes.SearchResult, error) {
	var rows []gen.SearchNotesRow
	err := r.inVault(ctx, slug, func(q *gen.Queries, vault publication.PublicVault) error {
		var err error
		rows, err = q.SearchNotes(ctx, gen.SearchNotesParams{
			SearchConfig: r.searchConfig, Query: query, VaultID: vault.VaultID, PageLimit: toInt32(limit), PageOffset: toInt32(offset),
		})
		return err
	})
	return convertRows(rows, toSearchResult), err
}

// PublicGraphVersion devolve a versão do grafo do cofre publicado com slug, zero antes
// do primeiro segundo estágio.
func (r *PostgresPublicVaults) PublicGraphVersion(ctx context.Context, slug string) (int64, error) {
	vault, err := r.PublicVault(ctx, slug)
	return vault.GraphVersion, err
}

// PublicGraph devolve o grafo do cofre publicado com slug recortado por f.
func (r *PostgresPublicVaults) PublicGraph(ctx context.Context, slug string, f notes.GraphFilter) (notes.Graph, error) {
	var out notes.Graph
	err := r.inVault(ctx, slug, func(q *gen.Queries, vault publication.PublicVault) error {
		nodes, err := listGraphNotes(ctx, q, vault.VaultID, nil, f)
		if err != nil {
			return err
		}
		out, err = assembleGraph(ctx, q, vault.VaultID, nodes, nil, nil, f)
		return err
	})
	return out, err
}

// PublicLinks devolve as ligações das notas do cofre publicado com slug.
func (r *PostgresPublicVaults) PublicLinks(ctx context.Context, slug string, vq notes.VaultLinkQuery) ([]notes.Link, error) {
	var out []notes.Link
	err := r.inVault(ctx, slug, func(q *gen.Queries, vault publication.PublicVault) error {
		rows, err := listVaultLinks(ctx, q, gen.ListVaultLinksParams{
			VaultID: vault.VaultID, Kind: vq.Type, PageOffset: toInt32(vq.Offset), PageLimit: toInt32(vq.Limit),
		}, vq.Status)
		out = convertRows(rows, toLink)
		return err
	})
	return out, err
}

// inVault roda read na transação pública com o cofre publicado com slug, ou devolve
// publication.ErrNotPublished: cofre inexistente e não publicado são o mesmo erro.
func (r *PostgresPublicVaults) inVault(ctx context.Context, slug string, read func(q *gen.Queries, vault publication.PublicVault) error) error {
	return inPublicTx(ctx, r.pool, func(q *gen.Queries) error {
		row, err := q.GetPublishedVault(ctx, slug)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: slug %q", publication.ErrNotPublished, slug)
		}
		if err != nil {
			return fmt.Errorf("ler o cofre publicado %q: %w", slug, err)
		}
		return read(q, publication.PublicVault{VaultID: row.VaultID, Slug: row.Slug, Title: row.Title, PublishedAt: row.PublishedAt})
	})
}
