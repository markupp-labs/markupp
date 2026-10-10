// Package users define as regras de negócio de contas e convites de usuários.
package users

import (
	"context"
	"errors"
	"time"
)

// Erros de domínio da gestão de usuários.
var (
	ErrUserNotFound        = errors.New("usuario nao encontrado")
	ErrUserAlreadyExists   = errors.New("usuario ja existe")
	ErrInviteNotFound      = errors.New("convite nao encontrado")
	ErrInviteAlreadyExists = errors.New("convite ja existe para este email")
	ErrOnlyAdminCanInvite  = errors.New("apenas administradores podem convidar usuarios")
	ErrInvalidEmail        = errors.New("email invalido")
	ErrEmptyTenant         = errors.New("tenant obrigatorio")
)

// User representa uma conta de usuário do sistema.
type User struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserInvite representa um convite enviado pelo administrador.
type UserInvite struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	InvitedBy string    `json:"invited_by"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Repository abstrai a persistência de contas e convites.
type Repository interface {
	CreateUser(ctx context.Context, u User) error
	CountUsersByTenant(ctx context.Context, tenantID string) (int64, error)
	GetUserByID(ctx context.Context, id string) (User, error)
	GetUserByEmail(ctx context.Context, tenantID, email string) (User, error)
	ListUsers(ctx context.Context, tenantID string) ([]User, error)
	CreateInvite(ctx context.Context, inv UserInvite) error
	GetInviteByEmail(ctx context.Context, tenantID, email string) (UserInvite, error)
	ListInvites(ctx context.Context, tenantID string) ([]UserInvite, error)
	UpdateInviteStatus(ctx context.Context, tenantID, email, status string) error
}
