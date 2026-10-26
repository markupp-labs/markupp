// Package publication publica o cofre para leitura sem conta (ADR-0048): valida o slug
// e o título, confere o papel de quem publica e audita cada publicação.
package publication

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

// MaxTitleLength é o maior título de publicação aceito, em caracteres.
const MaxTitleLength = 200

// Ações de auditoria da publicação.
const (
	ActionPublish   = "vaults.publish"
	ActionUnpublish = "vaults.unpublish"
)

// slugPattern é o mesmo CHECK de vault_publications.slug.
var slugPattern = regexp.MustCompile(`^[a-z0-9-]{3,63}$`)

// Erros da publicação.
var (
	ErrInvalidSlug  = errors.New("slug invalido")
	ErrInvalidTitle = errors.New("titulo invalido")
	ErrSlugTaken    = errors.New("slug ja usado por outro cofre")
	ErrNotPublished = errors.New("cofre nao publicado")
	ErrNotAdmin     = errors.New("publicacao reservada a owner e admin do tenant")
)

// Publication é o cofre publicado: o slug da URL pública, o título mostrado e quem
// publicou primeiro, e quando.
type Publication struct {
	VaultID     string
	Slug        string
	Title       string
	PublishedBy string
	PublishedAt time.Time
}

// PublicVault é o que a leitura sem conta mostra do cofre publicado, com a versão do
// grafo que valida o cache de quem lê.
type PublicVault struct {
	VaultID      string
	Slug         string
	Title        string
	PublishedAt  time.Time
	GraphVersion int64
}

// Repository grava a publicação no cofre do ator, auditada na mesma transação.
type Repository interface {
	// PublishTx grava ou troca o slug e o título do cofre p.VaultID. Devolve
	// ErrSlugTaken quando outro cofre tem o slug e notes.ErrVaultForbidden quando o
	// ator não enxerga o cofre.
	PublishTx(ctx context.Context, p Publication, event auth.AuditEventRecord) (Publication, error)
	// UnpublishTx apaga a publicação do cofre, ou devolve ErrNotPublished.
	UnpublishTx(ctx context.Context, vaultID string, event auth.AuditEventRecord) error
	// Publication lê a publicação do cofre, ou devolve ErrNotPublished.
	Publication(ctx context.Context, vaultID string) (Publication, error)
}

// Service publica e despublica os cofres do tenant do ator.
type Service struct {
	repo  Repository
	clock func() time.Time
	newID func() string
}

// NewService monta o serviço sobre o repositório e o relógio dados.
func NewService(repo Repository, clock func() time.Time) *Service {
	return &Service{repo: repo, clock: clock, newID: uuid.NewString}
}

// Publish publica o cofre vaultID com slug e title, ou troca os dois se ele já está
// publicado. Título vazio fica com o nome do cofre.
func (s *Service) Publish(ctx context.Context, actor identity.Actor, vaultID, slug, title string) (Publication, error) {
	if err := requireAdmin(actor); err != nil {
		return Publication{}, err
	}
	p, err := newPublication(actor, vaultID, slug, title, s.clock())
	if err != nil {
		return Publication{}, err
	}
	return s.repo.PublishTx(ctx, p, s.event(actor, ActionPublish, vaultID, p.PublishedAt))
}

// Unpublish tira o cofre vaultID da leitura sem conta.
func (s *Service) Unpublish(ctx context.Context, actor identity.Actor, vaultID string) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	return s.repo.UnpublishTx(ctx, vaultID, s.event(actor, ActionUnpublish, vaultID, s.clock()))
}

// Publication lê a publicação do cofre vaultID, que todo membro do cofre enxerga.
func (s *Service) Publication(ctx context.Context, vaultID string) (Publication, error) {
	return s.repo.Publication(ctx, vaultID)
}

func requireAdmin(actor identity.Actor) error {
	if actor.IsAdmin() {
		return nil
	}
	return fmt.Errorf("%w: papel %q, esperado owner ou admin", ErrNotAdmin, actor.Role)
}

func newPublication(actor identity.Actor, vaultID, slug, title string, now time.Time) (Publication, error) {
	if err := ValidateSlug(slug); err != nil {
		return Publication{}, err
	}
	title = strings.TrimSpace(title)
	if utf8.RuneCountInString(title) > MaxTitleLength {
		return Publication{}, fmt.Errorf("%w: %d caracteres, esperado no maximo %d", ErrInvalidTitle, utf8.RuneCountInString(title), MaxTitleLength)
	}
	return Publication{VaultID: vaultID, Slug: slug, Title: title, PublishedBy: actor.UserID, PublishedAt: now}, nil
}

// ValidateSlug aceita de 3 a 63 letras minúsculas, dígitos e hífens.
func ValidateSlug(slug string) error {
	if slugPattern.MatchString(slug) {
		return nil
	}
	return fmt.Errorf("%w: %q, esperado de 3 a 63 caracteres entre a-z, 0-9 e hifen", ErrInvalidSlug, slug)
}

func (s *Service) event(actor identity.Actor, action, vaultID string, at time.Time) auth.AuditEventRecord {
	return auth.AuditEventRecord{
		ID: s.newID(), TenantID: actor.TenantID, ActorID: actor.UserID, Action: action,
		Target: vaultID, Result: "success", OccurredAt: at,
	}
}
