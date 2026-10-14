package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

// inviteTTL é a validade do convite. Sem provedor de email, o administrador repassa
// o token à mão, e uma semana cobre esse repasse.
const inviteTTL = 7 * 24 * time.Hour

// Erros da administração de usuários.
var (
	ErrInvalidRole   = errors.New("papel invalido")
	ErrInvalidEmail  = errors.New("email invalido")
	ErrAlreadyMember = errors.New("email ja pertence a um usuario do tenant")
)

// Invite é o convite para entrar no tenant. O token não fica aqui: só o hash dele
// vai para o banco, e o token sai uma única vez, em IssuedInvite.
type Invite struct {
	ID         string
	TenantID   string
	Email      string
	Role       string
	InvitedBy  string
	ExpiresAt  time.Time
	AcceptedAt *time.Time
	CreatedAt  time.Time
}

// IssuedInvite é o convite recém-emitido com o token que o convidado apresenta.
type IssuedInvite struct {
	Invite
	Token string
}

// AdminRepository é a persistência da administração, sempre no tenant do ator.
type AdminRepository interface {
	ListUsers(ctx context.Context) ([]User, error)
	ListInvites(ctx context.Context) ([]Invite, error)
	CreateInviteTx(ctx context.Context, inv Invite, tokenHash string, event AuditEventRecord) error
}

// AdminService reúne o que o administrador faz no próprio tenant.
type AdminService struct {
	repo  AdminRepository
	clock func() time.Time
	newID func() string
}

// NewAdminService monta o serviço sobre o repositório e o relógio dados.
func NewAdminService(repo AdminRepository, clock func() time.Time) *AdminService {
	return &AdminService{repo: repo, clock: clock, newID: uuid.NewString}
}

// ListUsers devolve os usuários do tenant do ator.
func (s *AdminService) ListUsers(ctx context.Context) ([]User, error) {
	return s.repo.ListUsers(ctx)
}

// ListInvites devolve os convites do tenant do ator.
func (s *AdminService) ListInvites(ctx context.Context) ([]Invite, error) {
	return s.repo.ListInvites(ctx)
}

// IssueInvite grava o convite de email com o papel role, auditado na mesma
// transação, e devolve o token que o administrador repassa ao convidado.
func (s *AdminService) IssueInvite(ctx context.Context, inviter identity.Actor, email, role string) (IssuedInvite, error) {
	inv, err := s.newInvite(inviter, email, role)
	if err != nil {
		return IssuedInvite{}, err
	}
	token, err := generateRandomHex(32)
	if err != nil {
		return IssuedInvite{}, err
	}
	if err := s.repo.CreateInviteTx(ctx, inv, HashOpaqueToken(token), s.inviteEvent(inv)); err != nil {
		return IssuedInvite{}, fmt.Errorf("gravar convite de %q: %w", inv.Email, err)
	}
	return IssuedInvite{Invite: inv, Token: token}, nil
}

func (s *AdminService) newInvite(inviter identity.Actor, email, role string) (Invite, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(normEmail, "@") {
		return Invite{}, fmt.Errorf("%w: %q, esperado usuario@dominio", ErrInvalidEmail, email)
	}
	if role != identity.RoleAdmin && role != "member" {
		return Invite{}, fmt.Errorf("%w: %q, esperado admin ou member", ErrInvalidRole, role)
	}
	now := s.clock()
	return Invite{
		ID: s.newID(), TenantID: inviter.TenantID, Email: normEmail, Role: role,
		InvitedBy: inviter.UserID, ExpiresAt: now.Add(inviteTTL), CreatedAt: now,
	}, nil
}

func (s *AdminService) inviteEvent(inv Invite) AuditEventRecord {
	return AuditEventRecord{
		ID:         s.newID(),
		TenantID:   inv.TenantID,
		ActorID:    inv.InvitedBy,
		Action:     "users.invite",
		Target:     inv.Email,
		Result:     "success",
		OccurredAt: inv.CreatedAt,
	}
}
