package storage

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// ListVaultNotes devolve a página das notas do cofre vaultID que vq pede, com a
// organização derivada de cada uma, ou notes.ErrVaultForbidden quando o ator não
// enxerga o cofre.
func (r *PostgresNotesRepository) ListVaultNotes(ctx context.Context, vaultID string, vq notes.VaultNotesQuery) ([]notes.Note, error) {
	var out []notes.Note
	err := r.run(ctx, func(q *gen.Queries) error {
		if err := requireVisibleVault(ctx, q, vaultID); err != nil {
			return err
		}
		rows, err := q.ListVaultNotes(ctx, vaultNotesParams(vaultID, vq))
		out = convertRows(rows, toRankedNote)
		return err
	})
	return out, err
}

func vaultNotesParams(vaultID string, vq notes.VaultNotesQuery) gen.ListVaultNotesParams {
	p := gen.ListVaultNotesParams{VaultID: vaultID, Sort: vq.Sort, Descending: vq.Descending, PageOffset: toInt32(vq.Offset), PageLimit: toInt32(vq.Limit)}
	if vq.Orphan != nil {
		p.Orphan = pgtype.Bool{Bool: *vq.Orphan, Valid: true}
	}
	if vq.Community != nil {
		p.CommunityID = pgtype.Int8{Int64: *vq.Community, Valid: true}
	}
	return p
}

func toRankedNote(r gen.ListVaultNotesRow) notes.Note {
	note := toNote(gen.Note{ID: r.ID, VaultID: r.VaultID, Path: r.Path, Content: r.Content, ContentSha256: r.ContentSha256, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt})
	note.Rank = toNoteRank(gen.GetNoteRankRow{Pagerank: r.Pagerank, InDegree: r.InDegree, OutDegree: r.OutDegree, CommunityID: r.CommunityID})
	return note
}

// withRank lê a organização derivada da nota.
func withRank(ctx context.Context, q *gen.Queries, note notes.Note) (notes.Note, error) {
	row, err := q.GetNoteRank(ctx, note.ID)
	note.Rank = toNoteRank(row)
	return note, err
}

// toNoteRank converte as colunas nulas antes do segundo estágio; o grau soma os de
// entrada e de saída.
func toNoteRank(r gen.GetNoteRankRow) notes.NoteRank {
	var rank notes.NoteRank
	if r.Pagerank.Valid {
		rank.PageRank = &r.Pagerank.Float64
	}
	if r.InDegree.Valid && r.OutDegree.Valid {
		degree := int(r.InDegree.Int32 + r.OutDegree.Int32)
		rank.Degree = &degree
	}
	if r.CommunityID.Valid {
		rank.Community = &r.CommunityID.Int64
	}
	return rank
}
