package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// errTargetTaken marca o INSERT barrado pelo índice único do alvo, que aborta a
// transação: a publicação existente é lida noutra.
var errTargetTaken = errors.New("target already has a publication")

// PostgresPublications grava as publicações na transação do ator, sob o RLS da 00040.
type PostgresPublications struct {
	pool *pgxpool.Pool
}

// NewPostgresPublications monta o repositório sobre um pool já aberto.
func NewPostgresPublications(pool *pgxpool.Pool) *PostgresPublications {
	return &PostgresPublications{pool: pool}
}

// TargetInfo devolve o nome da confirmação e a contagem de notas do alvo no cofre vaultID.
func (r *PostgresPublications) TargetInfo(ctx context.Context, vaultID string, t publication.Target) (publication.TargetInfo, error) {
	var info publication.TargetInfo
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		if err := requireVisibleVault(ctx, q, vaultID); err != nil {
			return fmt.Errorf("publish in vault %q: %w", vaultID, err)
		}
		var err error
		info, err = targetInfo(ctx, q, vaultID, t)
		return err
	})
	return info, err
}

func targetInfo(ctx context.Context, q *gen.Queries, vaultID string, t publication.Target) (publication.TargetInfo, error) {
	switch t.Kind {
	case publication.KindNote:
		return noteTargetInfo(ctx, q, vaultID, t.NoteID)
	case publication.KindFolder:
		return folderTargetInfo(ctx, q, vaultID, t.FolderPath)
	}
	return vaultTargetInfo(ctx, q, vaultID)
}

func noteTargetInfo(ctx context.Context, q *gen.Queries, vaultID, noteID string) (publication.TargetInfo, error) {
	note, err := q.GetNoteByID(ctx, noteID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && note.VaultID != vaultID) {
		return publication.TargetInfo{}, fmt.Errorf("%w: note %q in vault %q", publication.ErrNoteNotFound, noteID, vaultID)
	}
	if err != nil {
		return publication.TargetInfo{}, fmt.Errorf("read note %q: %w", noteID, err)
	}
	return publication.TargetInfo{Name: indexing.Title(note.Path), NoteCount: 1}, nil
}

func folderTargetInfo(ctx context.Context, q *gen.Queries, vaultID, folder string) (publication.TargetInfo, error) {
	count, err := q.CountNotesUnder(ctx, gen.CountNotesUnderParams{VaultID: vaultID, FolderPath: folder})
	if err != nil {
		return publication.TargetInfo{}, fmt.Errorf("count notes under %q: %w", folder, err)
	}
	if count == 0 {
		return publication.TargetInfo{}, fmt.Errorf("%w: path %q in vault %q", publication.ErrFolderNotFound, folder, vaultID)
	}
	return publication.TargetInfo{Name: folder, NoteCount: int(count)}, nil
}

func vaultTargetInfo(ctx context.Context, q *gen.Queries, vaultID string) (publication.TargetInfo, error) {
	name, err := q.GetVaultName(ctx, vaultID)
	if err != nil {
		return publication.TargetInfo{}, fmt.Errorf("read vault %q: %w", vaultID, err)
	}
	count, err := q.CountNotesUnder(ctx, gen.CountNotesUnderParams{VaultID: vaultID})
	return publication.TargetInfo{Name: name, NoteCount: int(count)}, err
}

// InsertTx grava a publicação e a auditoria juntas e devolve a publicação lida de volta.
func (r *PostgresPublications) InsertTx(ctx context.Context, d publication.Draft, event auth.AuditEventRecord) (publication.Publication, error) {
	var out publication.Publication
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		if err := insertPublication(ctx, q, d); err != nil {
			return err
		}
		if err := q.CreateAuditEvent(ctx, toCreateAuditEventParams(event)); err != nil {
			return err
		}
		var err error
		out, err = readPublication(ctx, q, d.ID)
		return err
	})
	if errors.Is(err, errTargetTaken) {
		return out, r.alreadyPublished(ctx, d)
	}
	return out, err
}

func insertPublication(ctx context.Context, q *gen.Queries, d publication.Draft) error {
	err := q.InsertPublication(ctx, gen.InsertPublicationParams{
		ID: d.ID, VaultID: d.VaultID, Kind: d.Target.Kind, NoteID: nullableText(d.Target.NoteID),
		FolderPath: nullableText(d.Target.FolderPath), Indexable: d.Indexable, PublishedBy: d.PublishedBy, PublishedAt: d.PublishedAt,
	})
	switch {
	case isUniqueConstraintViolation(err):
		return errTargetTaken
	case pgErrorCode(err) == insufficientPrivilegeCode:
		return fmt.Errorf("%w: publication %q of %s in vault %q", publication.ErrPublishForbidden, d.ID, d.Target.Kind, d.VaultID)
	case err != nil:
		return fmt.Errorf("insert publication %q: %w", d.ID, err)
	}
	return nil
}

// alreadyPublished lê a publicação que já cobre o alvo de d.
func (r *PostgresPublications) alreadyPublished(ctx context.Context, d publication.Draft) error {
	return inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		id, err := q.FindTargetPublication(ctx, gen.FindTargetPublicationParams{
			VaultID: d.VaultID, Kind: d.Target.Kind, NoteID: nullableText(d.Target.NoteID), FolderPath: nullableText(d.Target.FolderPath),
		})
		if err != nil {
			return fmt.Errorf("read the publication of %s in vault %q: %w", d.Target.Kind, d.VaultID, err)
		}
		return &publication.AlreadyPublishedError{PublicationID: id}
	})
}

// List devolve as publicações que o ator enxerga, do cofre vaultID ou, vazio, do tenant.
func (r *PostgresPublications) List(ctx context.Context, vaultID string) ([]publication.Publication, error) {
	var rows []gen.ListPublicationsRow
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		var err error
		rows, err = q.ListPublications(ctx, gen.ListPublicationsParams{VaultID: vaultID})
		return err
	})
	return convertRows(rows, toPublication), err
}

// SetIndexableTx troca a indexação da publicação id e audita junto.
func (r *PostgresPublications) SetIndexableTx(ctx context.Context, id string, indexable bool, event auth.AuditEventRecord) (publication.Publication, error) {
	var out publication.Publication
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		n, err := q.SetPublicationIndexable(ctx, gen.SetPublicationIndexableParams{ID: id, Indexable: indexable})
		if err := requireChanged(ctx, q, id, n, err); err != nil {
			return err
		}
		if err := q.CreateAuditEvent(ctx, toCreateAuditEventParams(event)); err != nil {
			return err
		}
		out, err = readPublication(ctx, q, id)
		return err
	})
	return out, err
}

// DeleteTx apaga a publicação id e audita junto; o gatilho tira as notas que só ela cobria.
func (r *PostgresPublications) DeleteTx(ctx context.Context, id string, event auth.AuditEventRecord) error {
	return inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		n, err := q.DeletePublication(ctx, id)
		if err := requireChanged(ctx, q, id, n, err); err != nil {
			return err
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
}

// requireChanged explica a escrita que não mudou nenhuma linha: publicação que o ator
// não enxerga não existe para ele; a que ele enxerga, o RLS barrou pelo papel.
func requireChanged(ctx context.Context, q *gen.Queries, id string, changed int64, err error) error {
	if err != nil || changed > 0 {
		return err
	}
	visible, err := q.PublicationVisible(ctx, id)
	if err != nil {
		return fmt.Errorf("check publication %q: %w", id, err)
	}
	if visible {
		return fmt.Errorf("%w: publication %q", publication.ErrPublishForbidden, id)
	}
	return fmt.Errorf("%w: %q", publication.ErrPublicationNotFound, id)
}

func readPublication(ctx context.Context, q *gen.Queries, id string) (publication.Publication, error) {
	rows, err := q.ListPublications(ctx, gen.ListPublicationsParams{ID: id})
	if err == nil && len(rows) == 0 {
		err = pgx.ErrNoRows
	}
	if err != nil {
		return publication.Publication{}, fmt.Errorf("read publication %q: %w", id, err)
	}
	return toPublication(rows[0]), nil
}

func toPublication(r gen.ListPublicationsRow) publication.Publication {
	return publication.Publication{
		ID: r.ID, VaultID: r.VaultID, VaultName: r.VaultName, Kind: r.Kind, NoteID: r.NoteID, FolderPath: r.FolderPath,
		TargetName: targetName(r), NoteCount: int(r.NoteCount), Indexable: r.Indexable, PublishedByID: r.PublishedBy,
		PublishedByName: r.PublishedByEmail, PublishedAt: r.PublishedAt, PublicID: r.PublicID,
	}
}

// targetName é o nome que a confirmação pediu: título da nota, caminho da pasta ou nome
// do cofre.
func targetName(r gen.ListPublicationsRow) string {
	switch r.Kind {
	case publication.KindNote:
		return indexing.Title(r.NotePath)
	case publication.KindFolder:
		return r.FolderPath
	}
	return r.VaultName
}

// nullableText grava o texto vazio como NULL.
func nullableText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
