package users

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service gerencia regras de negócio para contas de usuários e convites.
type Service struct {
	repo  Repository
	clock func() time.Time
	newID func() string
}

// NewService monta um novo Service de usuários.
func NewService(repo Repository, clock func() time.Time) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{
		repo:  repo,
		clock: clock,
		newID: uuid.NewString,
	}
}

func normalizeEmail(email string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(norm, "@") {
		return "", ErrInvalidEmail
	}
	return norm, nil
}

func newUserData(id, tenantID, email, role string, t time.Time) User {
	return User{
		ID:        id,
		TenantID:  tenantID,
		Email:     email,
		Role:      role,
		CreatedAt: t,
		UpdatedAt: t,
	}
}

// BootstrapFirstAdmin cria a primeira conta como administrador caso o tenant não possua usuários.
func (s *Service) BootstrapFirstAdmin(ctx context.Context, tenantID, email string) (User, error) {
	normEmail, err := normalizeEmail(email)
	if err != nil {
		return User{}, err
	}
	count, err := s.repo.CountUsersByTenant(ctx, tenantID)
	if err != nil {
		return User{}, fmt.Errorf("contar usuarios: %w", err)
	}
	if count > 0 {
		return User{}, ErrUserAlreadyExists
	}

	now := s.clock().UTC().Truncate(time.Millisecond)
	u := newUserData(s.newID(), tenantID, normEmail, "admin", now)
	if err := s.repo.CreateUser(ctx, u); err != nil {
		return User{}, fmt.Errorf("criar admin: %w", err)
	}
	return u, nil
}

// InviteUser emite um convite para um novo usuário, permitido apenas a administradores.
func (s *Service) InviteUser(ctx context.Context, tenantID, actorRole, actorID, targetEmail, role string) (UserInvite, error) {
	if actorRole != "admin" {
		return UserInvite{}, ErrOnlyAdminCanInvite
	}
	normEmail, err := normalizeEmail(targetEmail)
	if err != nil {
		return UserInvite{}, err
	}
	if _, err := s.repo.GetUserByEmail(ctx, tenantID, normEmail); err == nil {
		return UserInvite{}, ErrUserAlreadyExists
	}

	now := s.clock().UTC().Truncate(time.Millisecond)
	inv := UserInvite{
		ID:        s.newID(),
		TenantID:  tenantID,
		Email:     normEmail,
		Role:      role,
		InvitedBy: actorID,
		Status:    "pending",
		CreatedAt: now,
	}
	if err := s.repo.CreateInvite(ctx, inv); err != nil {
		return UserInvite{}, err
	}
	return inv, nil
}

// AcceptInvite cria o usuário comum a partir de um convite pendente.
func (s *Service) AcceptInvite(ctx context.Context, tenantID, email string) (User, error) {
	normEmail, err := normalizeEmail(email)
	if err != nil {
		return User{}, err
	}
	inv, err := s.repo.GetInviteByEmail(ctx, tenantID, normEmail)
	if err != nil {
		return User{}, err
	}

	now := s.clock().UTC().Truncate(time.Millisecond)
	u := newUserData(s.newID(), tenantID, normEmail, inv.Role, now)
	if err := s.repo.CreateUser(ctx, u); err != nil {
		return User{}, fmt.Errorf("criar usuario por convite: %w", err)
	}
	if err := s.repo.UpdateInviteStatus(ctx, tenantID, normEmail, "accepted"); err != nil {
		return User{}, fmt.Errorf("atualizar convite: %w", err)
	}
	return u, nil
}

// ListUsers lista os usuários cadastrados do tenant.
func (s *Service) ListUsers(ctx context.Context, tenantID string) ([]User, error) {
	return s.repo.ListUsers(ctx, tenantID)
}

// ListInvites lista os convites emitidos para o tenant.
func (s *Service) ListInvites(ctx context.Context, tenantID string) ([]UserInvite, error) {
	return s.repo.ListInvites(ctx, tenantID)
}
