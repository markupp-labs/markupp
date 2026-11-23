// Package linkrefusals guarda a recusa de ligação: o par de notas de um cofre em que
// o servidor não cria ligação automática de nenhum tipo (ADR-0053).
package linkrefusals

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

// Erros da recusa de ligação.
var (
	ErrReadOnly         = errors.New("the viewer role can only read")
	ErrSameNote         = errors.New("a refusal needs two different notes")
	ErrNoteOutsideVault = errors.New("note not found in the vault")
	ErrRefusalNotFound  = errors.New("link refusal not found")
)

// Refusal é a recusa do par Notes, sem sentido, com as notas na ordem do id.
type Refusal struct {
	ID        string               `json:"id"`
	Notes     [2]notes.NoteSummary `json:"notes"`
	CreatedAt time.Time            `json:"created_at"`
}

// Store é a persistência das recusas, sempre nos cofres do ator do contexto. Create
// devolve a recusa que já existe para o par.
type Store interface {
	Create(ctx context.Context, vaultID, noteA, noteB string) (Refusal, error)
	List(ctx context.Context, vaultID string, p notes.Page) ([]Refusal, error)
	Delete(ctx context.Context, vaultID, id string) error
}

// Service barra o viewer e o par de uma nota só antes de chegar ao banco.
type Service struct {
	store Store
}

// NewService monta o serviço sobre store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// Refuse recusa a ligação automática entre noteA e noteB no cofre vaultID.
func (s *Service) Refuse(ctx context.Context, actor identity.Actor, vaultID, noteA, noteB string) (Refusal, error) {
	if err := checkWriter(actor); err != nil {
		return Refusal{}, err
	}
	if noteA == noteB {
		return Refusal{}, fmt.Errorf("%w: got %q twice", ErrSameNote, noteA)
	}
	return s.store.Create(ctx, vaultID, noteA, noteB)
}

// List devolve as recusas do cofre vaultID, da mais nova para a mais antiga.
func (s *Service) List(ctx context.Context, vaultID string, p notes.Page) ([]Refusal, error) {
	return s.store.List(ctx, vaultID, p)
}

// Undo desfaz a recusa id do cofre vaultID.
func (s *Service) Undo(ctx context.Context, actor identity.Actor, vaultID, id string) error {
	if err := checkWriter(actor); err != nil {
		return err
	}
	return s.store.Delete(ctx, vaultID, id)
}

func checkWriter(actor identity.Actor) error {
	if actor.Role == identity.RoleViewer {
		return fmt.Errorf("%w: role %q", ErrReadOnly, actor.Role)
	}
	return nil
}
