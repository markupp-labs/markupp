package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

// ErrLastOwner indica mudança que deixaria o tenant sem owner.
var ErrLastOwner = errors.New("o tenant ficaria sem owner")

// MembershipChange troca o papel de UserID para NewRole, ou remove o vínculo quando
// NewRole vem vazio, auditada por Event na mesma transação.
type MembershipChange struct {
	UserID  string
	NewRole string
	Event   AuditEventRecord
}

// MembershipState é o que a regra de mudança de vínculo lê do banco, com trava: o papel
// atual do membro alterado, o papel atual de quem altera e quantos owners o tenant tem.
type MembershipState struct {
	Current   string
	ActorRole string
	Owners    int
}

// CheckMembershipChange aplica as regras de quem mexe em quem: ninguém altera papel
// acima do seu nem concede papel acima do seu, e o tenant nunca fica sem owner.
func CheckMembershipChange(st MembershipState, newRole string) error {
	if identity.RoleRank(st.Current) > identity.RoleRank(st.ActorRole) {
		return fmt.Errorf("%w: %q mexendo em %q", ErrRoleAboveOwn, st.ActorRole, st.Current)
	}
	if newRole != "" {
		if err := checkRoleCeiling(identity.Actor{Role: st.ActorRole}, newRole); err != nil {
			return err
		}
	}
	if st.Current == identity.RoleOwner && newRole != identity.RoleOwner && st.Owners <= 1 {
		return fmt.Errorf("%w: %d owner restante", ErrLastOwner, st.Owners)
	}
	return nil
}

// ChangeRole troca o papel de userID no tenant do ator para role.
func (s *AdminService) ChangeRole(ctx context.Context, actor identity.Actor, userID, role string) error {
	if identity.RoleRank(role) == 0 {
		return fmt.Errorf("%w: %q, esperado owner, admin, member ou viewer", ErrInvalidRole, role)
	}
	return s.changeMembership(ctx, actor, userID, role, "members.role")
}

// RemoveMember tira userID do tenant do ator.
func (s *AdminService) RemoveMember(ctx context.Context, actor identity.Actor, userID string) error {
	return s.changeMembership(ctx, actor, userID, "", "members.remove")
}

func (s *AdminService) changeMembership(ctx context.Context, actor identity.Actor, userID, role, action string) error {
	change := MembershipChange{UserID: userID, NewRole: role, Event: AuditEventRecord{
		ID: s.newID(), TenantID: actor.TenantID, ActorID: actor.UserID, Action: action,
		Target: userID, Result: "success", OccurredAt: s.clock(),
	}}
	return s.repo.ChangeMembershipTx(ctx, change, func(st MembershipState) error {
		return CheckMembershipChange(st, role)
	})
}
