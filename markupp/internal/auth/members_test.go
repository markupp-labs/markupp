package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

func (f *fakeAdminRepository) ChangeMembershipTx(ctx context.Context, c auth.MembershipChange, check func(auth.MembershipState) error) error {
	f.change = c
	return check(auth.MembershipState{Current: f.currentRole, ActorRole: f.actorRole, Owners: f.owners})
}

func TestCheckMembershipChange_Regras(t *testing.T) {
	casos := []struct {
		nome              string
		ator, atual, novo string
		owners            int
		esperado          error
	}{
		{"admin rebaixa member", "admin", "member", "viewer", 1, nil},
		{"admin promove a admin", "admin", "member", "admin", 1, nil},
		{"admin promove a owner", "admin", "member", "owner", 1, auth.ErrRoleAboveOwn},
		{"admin mexe em owner", "admin", "owner", "member", 2, auth.ErrRoleAboveOwn},
		{"owner rebaixa outro owner", "owner", "owner", "admin", 2, nil},
		{"owner rebaixa o ultimo owner", "owner", "owner", "admin", 1, auth.ErrLastOwner},
		{"owner remove o ultimo owner", "owner", "owner", "", 1, auth.ErrLastOwner},
		{"admin remove member", "admin", "member", "", 1, nil},
		{"admin remove admin", "admin", "admin", "", 1, nil},
	}
	for _, c := range casos {
		err := auth.CheckMembershipChange(auth.MembershipState{Current: c.atual, ActorRole: c.ator, Owners: c.owners}, c.novo)

		if c.esperado == nil {
			assert.NoError(t, err, c.nome)
			continue
		}
		assert.ErrorIs(t, err, c.esperado, c.nome)
	}
}

func TestChangeRole_AplicaARegraComOEstadoDoBanco(t *testing.T) {
	repo := &fakeAdminRepository{currentRole: "owner", actorRole: "owner", owners: 1}
	svc := auth.NewAdminService(repo, defaultClock())

	err := svc.ChangeRole(context.Background(), identity.Actor{UserID: "dona", TenantID: "t1", Role: "owner"}, "dona", "member")

	require.ErrorIs(t, err, auth.ErrLastOwner)
	assert.Equal(t, "dona", repo.change.UserID)
	assert.Equal(t, "member", repo.change.NewRole)
	assert.Equal(t, "members.role", repo.change.Event.Action)
}

func TestChangeRole_PapelInexistente_RecusaAntesDoBanco(t *testing.T) {
	repo := &fakeAdminRepository{}

	err := auth.NewAdminService(repo, defaultClock()).ChangeRole(context.Background(), adminDeTeste, "u2", "root")

	require.ErrorIs(t, err, auth.ErrInvalidRole)
	assert.Empty(t, repo.change.UserID)
}

func TestRemoveMember_AuditaComoRemocao(t *testing.T) {
	repo := &fakeAdminRepository{currentRole: "member", actorRole: "admin", owners: 1}

	err := auth.NewAdminService(repo, defaultClock()).RemoveMember(context.Background(), adminDeTeste, "u2")

	require.NoError(t, err)
	assert.Empty(t, repo.change.NewRole, "papel vazio significa remover")
	assert.Equal(t, "members.remove", repo.change.Event.Action)
	assert.Equal(t, "u2", repo.change.Event.Target)
	assert.Equal(t, "t1", repo.change.Event.TenantID)
}

func TestChangeRole_UsaOPapelDoAtorLidoNoBanco(t *testing.T) {
	repo := &fakeAdminRepository{currentRole: "member", actorRole: "member", owners: 1}

	err := auth.NewAdminService(repo, defaultClock()).ChangeRole(context.Background(),
		identity.Actor{UserID: "ex-owner", TenantID: "t1", Role: "owner"}, "u2", "owner")

	require.ErrorIs(t, err, auth.ErrRoleAboveOwn, "o papel da requisicao pode estar velho; vale o do banco")
}
