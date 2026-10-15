package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

type fakeAdminRepository struct {
	invite    auth.Invite
	tokenHash string
	event     auth.AuditEventRecord
	createErr error

	change      auth.MembershipChange
	currentRole string
	actorRole   string
	owners      int
	revoked     string
}

func (f *fakeAdminRepository) ListMembers(ctx context.Context) ([]auth.Member, error) {
	return nil, nil
}

func (f *fakeAdminRepository) ListInvites(ctx context.Context) ([]auth.Invite, error) {
	return nil, nil
}

func (f *fakeAdminRepository) CreateInviteTx(ctx context.Context, inv auth.Invite, tokenHash string, event auth.AuditEventRecord) error {
	f.invite, f.tokenHash, f.event = inv, tokenHash, event
	return f.createErr
}

var adminDeTeste = identity.Actor{UserID: "admin-1", TenantID: "t1", Role: identity.RoleAdmin}

func TestIssueInvite_DadosValidos_GuardaSoOHashEAudita(t *testing.T) {
	repo := &fakeAdminRepository{}
	clock := defaultClock()
	svc := auth.NewAdminService(repo, clock)

	issued, err := svc.IssueInvite(context.Background(), adminDeTeste, "  Nova@Markupp.dev ", "member")

	require.NoError(t, err)
	assert.NotEmpty(t, issued.Token)
	assert.Equal(t, auth.HashOpaqueToken(issued.Token), repo.tokenHash)
	assert.NotContains(t, repo.tokenHash, issued.Token)
	assert.Equal(t, "nova@markupp.dev", repo.invite.Email)
	assert.Equal(t, "member", repo.invite.Role)
	assert.Equal(t, "t1", repo.invite.TenantID)
	assert.Equal(t, "admin-1", repo.invite.InvitedBy)
	assert.Equal(t, clock().Add(7*24*time.Hour), repo.invite.ExpiresAt)
	assert.Equal(t, auth.AuditEventRecord{
		ID: repo.event.ID, TenantID: "t1", ActorID: "admin-1", Action: "users.invite",
		Target: "nova@markupp.dev", Result: "success", OccurredAt: clock(),
	}, repo.event)
	assert.NotEmpty(t, repo.event.ID)
}

func TestIssueInvite_PapelForaDaLista_RecusaComOValor(t *testing.T) {
	svc := auth.NewAdminService(&fakeAdminRepository{}, defaultClock())

	_, err := svc.IssueInvite(context.Background(), adminDeTeste, "nova@markupp.dev", "root")

	require.ErrorIs(t, err, auth.ErrInvalidRole)
	assert.Contains(t, err.Error(), `"root"`)
}

func TestIssueInvite_EmailSemArroba_RecusaComOValor(t *testing.T) {
	svc := auth.NewAdminService(&fakeAdminRepository{}, defaultClock())

	_, err := svc.IssueInvite(context.Background(), adminDeTeste, "semarroba", "member")

	require.ErrorIs(t, err, auth.ErrInvalidEmail)
	assert.Contains(t, err.Error(), `"semarroba"`)
}

func TestIssueInvite_EmailJaEMembro_PropagaErrAlreadyMember(t *testing.T) {
	svc := auth.NewAdminService(&fakeAdminRepository{createErr: auth.ErrAlreadyMember}, defaultClock())

	_, err := svc.IssueInvite(context.Background(), adminDeTeste, "velho@markupp.dev", "admin")

	require.ErrorIs(t, err, auth.ErrAlreadyMember)
}

func TestIssueInvite_PapelAteODeQuemConvida_Aceito(t *testing.T) {
	for _, papel := range []string{"admin", "member", "viewer"} {
		repo := &fakeAdminRepository{}

		_, err := auth.NewAdminService(repo, defaultClock()).IssueInvite(context.Background(), adminDeTeste, "nova@markupp.dev", papel)

		require.NoError(t, err, papel)
		assert.Equal(t, papel, repo.invite.Role)
	}
}

func TestIssueInvite_PapelAcimaDoDeQuemConvida_Recusa(t *testing.T) {
	_, err := auth.NewAdminService(&fakeAdminRepository{}, defaultClock()).IssueInvite(context.Background(), adminDeTeste, "nova@markupp.dev", "owner")

	require.ErrorIs(t, err, auth.ErrRoleAboveOwn)
	assert.Contains(t, err.Error(), `"owner"`)
}

func TestIssueInvite_OwnerConvidaOwner(t *testing.T) {
	dona := identity.Actor{UserID: "dona", TenantID: "t1", Role: "owner"}

	_, err := auth.NewAdminService(&fakeAdminRepository{}, defaultClock()).IssueInvite(context.Background(), dona, "socia@markupp.dev", "owner")

	require.NoError(t, err)
}

func (f *fakeAdminRepository) RevokeInviteTx(ctx context.Context, inviteID string, event auth.AuditEventRecord) error {
	f.revoked, f.event = inviteID, event
	return nil
}

func TestRevokeInvite_AuditaARevogacao(t *testing.T) {
	repo := &fakeAdminRepository{}

	require.NoError(t, auth.NewAdminService(repo, defaultClock()).RevokeInvite(context.Background(), adminDeTeste, "c1"))

	assert.Equal(t, "c1", repo.revoked)
	assert.Equal(t, "users.invite_revoke", repo.event.Action)
	assert.Equal(t, "t1", repo.event.TenantID)
	assert.Equal(t, "admin-1", repo.event.ActorID)
}
