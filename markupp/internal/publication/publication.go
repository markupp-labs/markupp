// Package publication publica notas, pastas e cofres para leitura sem conta (ADR-0048):
// valida o alvo, confere o papel de quem publica e o nome digitado na confirmação, e
// audita cada mudança.
package publication

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

// Alvos da publicação.
const (
	KindNote   = "note"
	KindFolder = "folder"
	KindVault  = "vault"
)

// Ações de auditoria da publicação.
const (
	ActionPublish   = "publications.publish"
	ActionUnpublish = "publications.unpublish"
	ActionUpdate    = "publications.update"
)

// folderPattern é o mesmo CHECK de publications.folder_path.
var folderPattern = regexp.MustCompile(`^[^/]+(/[^/]+)*$`)

// Erros da publicação.
var (
	ErrInvalidTarget        = errors.New("invalid publication target")
	ErrConfirmationMismatch = errors.New("confirmation does not match the target name")
	ErrPublishForbidden     = errors.New("account cannot change the publication of this target")
	ErrNoteNotFound         = errors.New("note not found in the vault")
	ErrFolderNotFound       = errors.New("folder has no notes in the vault")
	ErrAlreadyPublished     = errors.New("target already published")
	ErrPublicationNotFound  = errors.New("publication not found")
	ErrPublicNoteNotFound   = errors.New("public note not found")
)

// AlreadyPublishedError é o ErrAlreadyPublished com a publicação que já cobre o alvo.
type AlreadyPublishedError struct {
	PublicationID string
}

func (e *AlreadyPublishedError) Error() string {
	return fmt.Sprintf("%v: publication %q", ErrAlreadyPublished, e.PublicationID)
}

func (e *AlreadyPublishedError) Unwrap() error { return ErrAlreadyPublished }

// Target é o que se publica: a nota NoteID, a pasta FolderPath ou o cofre inteiro.
type Target struct {
	Kind       string
	NoteID     string
	FolderPath string
}

// Validate confere o kind e o campo que ele pede.
func (t Target) Validate() error {
	switch t.Kind {
	case KindVault:
		return nil
	case KindNote:
		if t.NoteID == "" {
			return fmt.Errorf("%w: note_id is empty, expected the id of a note in the vault", ErrInvalidTarget)
		}
		return nil
	case KindFolder:
		if !folderPattern.MatchString(t.FolderPath) {
			return fmt.Errorf("%w: path %q, expected a folder like docs/deploy, without leading or trailing slash", ErrInvalidTarget, t.FolderPath)
		}
		return nil
	}
	return fmt.Errorf("%w: kind %q, expected note, folder or vault", ErrInvalidTarget, t.Kind)
}

// TargetInfo é o nome que a confirmação pede (título da nota, caminho da pasta ou nome do
// cofre) e quantas notas o alvo cobre hoje.
type TargetInfo struct {
	Name      string
	NoteCount int
}

// Preview é o que o pedido de confirmação mostra antes de publicar.
type Preview struct {
	Kind           string
	TargetName     string
	NoteCount      int
	IncludesFuture bool
}

// Request é o pedido de publicação, com o nome digitado na confirmação.
type Request struct {
	Target    Target
	Confirm   string
	Indexable bool
}

// Draft é a publicação a gravar.
type Draft struct {
	ID          string
	VaultID     string
	Target      Target
	Indexable   bool
	PublishedBy string
	PublishedAt time.Time
}

// Publication é a publicação gravada, com os nomes do cofre, do alvo e de quem publicou.
// PublicID só existe na publicação de nota.
type Publication struct {
	ID              string
	VaultID         string
	VaultName       string
	Kind            string
	NoteID          string
	FolderPath      string
	TargetName      string
	NoteCount       int
	Indexable       bool
	PublishedByID   string
	PublishedByName string
	PublishedAt     time.Time
	PublicID        string
}

// Repository grava as publicações no tenant do ator, auditadas na mesma transação.
type Repository interface {
	// TargetInfo devolve o nome e a contagem do alvo, ou notes.ErrVaultForbidden,
	// ErrNoteNotFound ou ErrFolderNotFound.
	TargetInfo(ctx context.Context, vaultID string, t Target) (TargetInfo, error)
	// InsertTx grava a publicação, ou devolve *AlreadyPublishedError ou ErrPublishForbidden.
	InsertTx(ctx context.Context, d Draft, event auth.AuditEventRecord) (Publication, error)
	// List devolve as publicações que o ator enxerga, do cofre vaultID ou, vazio, do tenant.
	List(ctx context.Context, vaultID string) ([]Publication, error)
	// SetIndexableTx e DeleteTx devolvem ErrPublicationNotFound ou ErrPublishForbidden.
	SetIndexableTx(ctx context.Context, id string, indexable bool, event auth.AuditEventRecord) (Publication, error)
	DeleteTx(ctx context.Context, id string, event auth.AuditEventRecord) error
}

// Service publica e despublica no tenant do ator.
type Service struct {
	repo  Repository
	clock func() time.Time
	newID func() string
}

// NewService monta o serviço sobre o repositório, o relógio e o gerador de id dados.
func NewService(repo Repository, clock func() time.Time, newID func() string) *Service {
	return &Service{repo: repo, clock: clock, newID: newID}
}

// Preview diz o que publicar t no cofre vaultID abre para leitura sem conta.
func (s *Service) Preview(ctx context.Context, vaultID string, t Target) (Preview, error) {
	if err := t.Validate(); err != nil {
		return Preview{}, err
	}
	info, err := s.repo.TargetInfo(ctx, vaultID, t)
	return Preview{Kind: t.Kind, TargetName: info.Name, NoteCount: info.NoteCount, IncludesFuture: t.Kind != KindNote}, err
}

// Publish publica o alvo do pedido no cofre vaultID quando a confirmação traz o nome dele.
func (s *Service) Publish(ctx context.Context, actor identity.Actor, vaultID string, req Request) (Publication, error) {
	if err := s.checkRequest(ctx, actor, vaultID, req); err != nil {
		return Publication{}, err
	}
	d := Draft{ID: s.newID(), VaultID: vaultID, Target: req.Target, Indexable: req.Indexable, PublishedBy: actor.UserID, PublishedAt: s.clock()}
	return s.repo.InsertTx(ctx, d, s.event(actor, ActionPublish, d.ID, d.PublishedAt))
}

// checkRequest confere, nesta ordem, o formato do alvo, o papel, o alvo no cofre e o nome.
func (s *Service) checkRequest(ctx context.Context, actor identity.Actor, vaultID string, req Request) error {
	if err := req.Target.Validate(); err != nil {
		return err
	}
	if err := requirePublisher(actor, req.Target.Kind); err != nil {
		return err
	}
	info, err := s.repo.TargetInfo(ctx, vaultID, req.Target)
	if err != nil {
		return err
	}
	if strings.TrimSpace(req.Confirm) != info.Name {
		return fmt.Errorf("%w: received %q, expected %q", ErrConfirmationMismatch, req.Confirm, info.Name)
	}
	return nil
}

// List devolve as publicações que o ator enxerga, do cofre vaultID ou, vazio, do tenant.
func (s *Service) List(ctx context.Context, vaultID string) ([]Publication, error) {
	return s.repo.List(ctx, vaultID)
}

// SetIndexable liga ou desliga a indexação por buscadores da publicação id.
func (s *Service) SetIndexable(ctx context.Context, actor identity.Actor, id string, indexable bool) (Publication, error) {
	if err := requireEditor(actor); err != nil {
		return Publication{}, err
	}
	return s.repo.SetIndexableTx(ctx, id, indexable, s.event(actor, ActionUpdate, id, s.clock()))
}

// Unpublish apaga a publicação id, e as notas que só ela cobria ficam privadas.
func (s *Service) Unpublish(ctx context.Context, actor identity.Actor, id string) error {
	if err := requireEditor(actor); err != nil {
		return err
	}
	return s.repo.DeleteTx(ctx, id, s.event(actor, ActionUnpublish, id, s.clock()))
}

// requirePublisher barra o viewer, e quem não é owner nem admin na pasta e no cofre.
func requirePublisher(actor identity.Actor, kind string) error {
	if err := requireEditor(actor); err != nil {
		return err
	}
	if kind == KindNote || actor.IsAdmin() {
		return nil
	}
	return fmt.Errorf("%w: role %q publishing a %s, expected owner or admin", ErrPublishForbidden, actor.Role, kind)
}

func requireEditor(actor identity.Actor) error {
	if actor.Role != identity.RoleViewer {
		return nil
	}
	return fmt.Errorf("%w: role %q, expected a role that edits notes", ErrPublishForbidden, actor.Role)
}

func (s *Service) event(actor identity.Actor, action, publicationID string, at time.Time) auth.AuditEventRecord {
	return auth.AuditEventRecord{
		ID: s.newID(), TenantID: actor.TenantID, ActorID: actor.UserID, Action: action,
		Target: publicationID, Result: "success", OccurredAt: at,
	}
}
