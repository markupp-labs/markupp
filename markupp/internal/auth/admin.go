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
	ErrRoleAboveOwn  = errors.New("papel acima do de quem atribui")
	ErrNotOwner      = errors.New("acao reservada ao owner do tenant")
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
	ListMembers(ctx context.Context) ([]Member, error)
	ListInvites(ctx context.Context) ([]Invite, error)
	// CreateInviteTx grava o convite e a auditoria e, com byEmail, a notificação que
	// entrega o token por email, tudo na mesma transação.
	CreateInviteTx(ctx context.Context, inv Invite, tokenHash string, event AuditEventRecord, byEmail bool) error
	ChangeMembershipTx(ctx context.Context, change MembershipChange, check func(MembershipState) error) error
	RevokeInviteTx(ctx context.Context, inviteID string, event AuditEventRecord) error
	// DeleteTenantTx apaga de vez o tenant do ator, com tudo que pertence a ele, e grava
	// a auditoria na mesma transação.
	DeleteTenantTx(ctx context.Context, event AuditEventRecord) error
}

// AdminService reúne o que o administrador faz no próprio tenant.
type AdminService struct {
	repo          AdminRepository
	clock         func() time.Time
	newID         func() string
	inviteByEmail bool
}

// NewAdminService monta o serviço sobre o repositório e o relógio dados. Com
// inviteByEmail, o convite sai por email e o token nunca passa pela resposta.
func NewAdminService(repo AdminRepository, clock func() time.Time, inviteByEmail bool) *AdminService {
	return &AdminService{repo: repo, clock: clock, newID: uuid.NewString, inviteByEmail: inviteByEmail}
}

// ListMembers devolve os membros do tenant do ator, com o papel de cada um.
func (s *AdminService) ListMembers(ctx context.Context) ([]Member, error) {
	return s.repo.ListMembers(ctx)
}

// ListInvites devolve os convites do tenant do ator.
func (s *AdminService) ListInvites(ctx context.Context) ([]Invite, error) {
	return s.repo.ListInvites(ctx)
}

// IssueInvite grava o convite de email com o papel role, auditado na mesma
// transação. Sem email, devolve o token que o administrador repassa ao convidado.
// Com email, o token desta emissão é descartado: o serviço de notificação gera
// outro na entrega, e o convidado só recebe esse.
func (s *AdminService) IssueInvite(ctx context.Context, inviter identity.Actor, email, role string) (IssuedInvite, error) {
	inv, err := s.newInvite(inviter, email, role)
	if err != nil {
		return IssuedInvite{}, err
	}
	token, err := NewOpaqueToken()
	if err != nil {
		return IssuedInvite{}, err
	}
	if err := s.repo.CreateInviteTx(ctx, inv, HashOpaqueToken(token), s.inviteEvent(inv), s.inviteByEmail); err != nil {
		return IssuedInvite{}, fmt.Errorf("gravar convite de %q: %w", inv.Email, err)
	}
	if s.inviteByEmail {
		return IssuedInvite{Invite: inv}, nil
	}
	return IssuedInvite{Invite: inv, Token: token}, nil
}

func (s *AdminService) newInvite(inviter identity.Actor, email, role string) (Invite, error) {
	normEmail := normalizeEmail(email)
	if !strings.Contains(normEmail, "@") {
		return Invite{}, fmt.Errorf("%w: %q, esperado usuario@dominio", ErrInvalidEmail, email)
	}
	if identity.RoleRank(role) == 0 {
		return Invite{}, fmt.Errorf("%w: %q, esperado owner, admin, member ou viewer", ErrInvalidRole, role)
	}
	if err := checkRoleCeiling(inviter, role); err != nil {
		return Invite{}, err
	}
	now := s.clock()
	return Invite{
		ID: s.newID(), TenantID: inviter.TenantID, Email: normEmail, Role: role,
		InvitedBy: inviter.UserID, ExpiresAt: now.Add(inviteTTL), CreatedAt: now,
	}, nil
}

// checkRoleCeiling impede que alguém conceda papel acima do próprio.
func checkRoleCeiling(granter identity.Actor, role string) error {
	if identity.RoleRank(role) > identity.RoleRank(granter.Role) {
		return fmt.Errorf("%w: %q por quem e %q", ErrRoleAboveOwn, role, granter.Role)
	}
	return nil
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

// RevokeInvite cancela o convite inviteID do tenant do ator antes da resposta.
func (s *AdminService) RevokeInvite(ctx context.Context, actor identity.Actor, inviteID string) error {
	return s.repo.RevokeInviteTx(ctx, inviteID, AuditEventRecord{
		ID: s.newID(), TenantID: actor.TenantID, ActorID: actor.UserID, Action: "users.invite_revoke",
		Target: inviteID, Result: "success", OccurredAt: s.clock(),
	})
}

// DeleteTenant apaga de vez o tenant do ator, com cofres, notas, vínculos e convites,
// auditado na mesma transação. Só o owner apaga; as contas continuam.
func (s *AdminService) DeleteTenant(ctx context.Context, actor identity.Actor) error {
	if actor.Role != identity.RoleOwner {
		return fmt.Errorf("%w: papel %q, esperado %q", ErrNotOwner, actor.Role, identity.RoleOwner)
	}
	return s.repo.DeleteTenantTx(ctx, AuditEventRecord{
		ID: s.newID(), TenantID: actor.TenantID, ActorID: actor.UserID, Action: "tenants.delete",
		Target: actor.TenantID, Result: "success", OccurredAt: s.clock(),
	})
}
