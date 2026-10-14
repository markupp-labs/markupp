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
}

func (f *fakeAdminRepository) ListUsers(ctx context.Context) ([]auth.User, error) {
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
