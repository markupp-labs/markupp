package vaults

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

// maxNameLength é o mesmo limite do nome do tenant, que também aparece num seletor.
const maxNameLength = 100

// Repository é a persistência da gestão de cofres, sempre no tenant e nos cofres do
// ator; cada escrita grava a auditoria na mesma transação.
type Repository interface {
	List(ctx context.Context) ([]Vault, error)
	CreateTx(ctx context.Context, v Vault, event auth.AuditEventRecord) error
	RenameTx(ctx context.Context, id, name string, event auth.AuditEventRecord) (Vault, error)
	DeleteTx(ctx context.Context, id string, event auth.AuditEventRecord) error
	ListMembers(ctx context.Context, vaultID string) ([]Member, error)
	AddMemberTx(ctx context.Context, vaultID, userID string, at time.Time, event auth.AuditEventRecord) (Member, error)
	RemoveMemberTx(ctx context.Context, vaultID, userID string, event auth.AuditEventRecord) error
}

// Service aplica as regras da gestão de cofres: nome válido e viewer só sai.
type Service struct {
	repo  Repository
	clock func() time.Time
	newID func() string
}

// NewService monta o serviço sobre o repositório, o relógio e o gerador de id dados.
func NewService(repo Repository, clock func() time.Time, newID func() string) *Service {
	return &Service{repo: repo, clock: clock, newID: newID}
}

// List devolve os cofres de que o ator é membro.
func (s *Service) List(ctx context.Context) ([]Vault, error) {
	return s.repo.List(ctx)
}

// ListMembers devolve os membros do cofre vaultID.
func (s *Service) ListMembers(ctx context.Context, vaultID string) ([]Member, error) {
	return s.repo.ListMembers(ctx, vaultID)
}

// Create grava o cofre name com o ator como primeiro membro.
func (s *Service) Create(ctx context.Context, actor identity.Actor, name string) (Vault, error) {
	name, err := checkWrite(actor, name)
	if err != nil {
		return Vault{}, err
	}
	v := Vault{ID: s.newID(), Name: name, CreatedAt: s.clock()}
	if err := s.repo.CreateTx(ctx, v, s.event(actor, "vaults.create", v.ID)); err != nil {
		return Vault{}, err
	}
	return v, nil
}

// Rename troca o nome do cofre id para name.
func (s *Service) Rename(ctx context.Context, actor identity.Actor, id, name string) (Vault, error) {
	name, err := checkWrite(actor, name)
	if err != nil {
		return Vault{}, err
	}
	return s.repo.RenameTx(ctx, id, name, s.event(actor, "vaults.rename", id))
}

// Delete apaga o cofre id com as notas dele.
func (s *Service) Delete(ctx context.Context, actor identity.Actor, id string) error {
	if err := checkWriter(actor); err != nil {
		return err
	}
	return s.repo.DeleteTx(ctx, id, s.event(actor, "vaults.delete", id))
}

// AddMember põe a conta userID, que precisa ser do tenant, no cofre vaultID.
func (s *Service) AddMember(ctx context.Context, actor identity.Actor, vaultID, userID string) (Member, error) {
	if err := checkWriter(actor); err != nil {
		return Member{}, err
	}
	return s.repo.AddMemberTx(ctx, vaultID, userID, s.clock(), s.event(actor, "vaults.members.add", userID))
}

// RemoveMember tira userID do cofre vaultID. Tirar a si mesmo é sair, e até o viewer sai.
func (s *Service) RemoveMember(ctx context.Context, actor identity.Actor, vaultID, userID string) error {
	if userID != actor.UserID {
		if err := checkWriter(actor); err != nil {
			return err
		}
	}
	return s.repo.RemoveMemberTx(ctx, vaultID, userID, s.event(actor, "vaults.members.remove", userID))
}

func (s *Service) event(actor identity.Actor, action, target string) auth.AuditEventRecord {
	return auth.AuditEventRecord{
		ID: s.newID(), TenantID: actor.TenantID, ActorID: actor.UserID, Action: action,
		Target: target, Result: "success", OccurredAt: s.clock(),
	}
}

// checkWrite barra o viewer e devolve o nome aparado, de 1 a maxNameLength caracteres.
func checkWrite(actor identity.Actor, name string) (string, error) {
	if err := checkWriter(actor); err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || len(trimmed) > maxNameLength {
		return "", fmt.Errorf("%w: %q, esperado de 1 a %d caracteres", ErrInvalidName, name, maxNameLength)
	}
	return trimmed, nil
}

func checkWriter(actor identity.Actor) error {
	if actor.Role == identity.RoleViewer {
		return fmt.Errorf("%w: papel %q", ErrReadOnly, actor.Role)
	}
	return nil
}
