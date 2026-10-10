package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/users"
)

type fakeUsersRepository struct {
	users           map[string]users.User
	invites         map[string]users.UserInvite
	createErr       error
	countErr        error
	getInviteErr    error
	updateStatusErr error
}

func newFakeUsersRepository() *fakeUsersRepository {
	return &fakeUsersRepository{
		users:   make(map[string]users.User),
		invites: make(map[string]users.UserInvite),
	}
}

func (f *fakeUsersRepository) userKey(tenantID, email string) string {
	return tenantID + ":" + email
}

func (f *fakeUsersRepository) CreateUser(ctx context.Context, u users.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	key := f.userKey(u.TenantID, u.Email)
	if _, exists := f.users[key]; exists {
		return users.ErrUserAlreadyExists
	}
	f.users[key] = u
	return nil
}

func (f *fakeUsersRepository) CountUsersByTenant(ctx context.Context, tenantID string) (int64, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	var count int64
	for _, u := range f.users {
		if u.TenantID == tenantID {
			count++
		}
	}
	return count, nil
}

func (f *fakeUsersRepository) GetUserByID(ctx context.Context, id string) (users.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return users.User{}, users.ErrUserNotFound
}

func (f *fakeUsersRepository) GetUserByEmail(ctx context.Context, tenantID, email string) (users.User, error) {
	u, ok := f.users[f.userKey(tenantID, email)]
	if !ok {
		return users.User{}, users.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUsersRepository) ListUsers(ctx context.Context, tenantID string) ([]users.User, error) {
	var list []users.User
	for _, u := range f.users {
		if u.TenantID == tenantID {
			list = append(list, u)
		}
	}
	return list, nil
}

func (f *fakeUsersRepository) CreateInvite(ctx context.Context, inv users.UserInvite) error {
	key := f.userKey(inv.TenantID, inv.Email)
	if _, exists := f.invites[key]; exists {
		return users.ErrInviteAlreadyExists
	}
	f.invites[key] = inv
	return nil
}

func (f *fakeUsersRepository) GetInviteByEmail(ctx context.Context, tenantID, email string) (users.UserInvite, error) {
	if f.getInviteErr != nil {
		return users.UserInvite{}, f.getInviteErr
	}
	inv, ok := f.invites[f.userKey(tenantID, email)]
	if !ok {
		return users.UserInvite{}, users.ErrInviteNotFound
	}
	return inv, nil
}

func (f *fakeUsersRepository) ListInvites(ctx context.Context, tenantID string) ([]users.UserInvite, error) {
	var list []users.UserInvite
	for _, inv := range f.invites {
		if inv.TenantID == tenantID {
			list = append(list, inv)
		}
	}
	return list, nil
}

func (f *fakeUsersRepository) UpdateInviteStatus(ctx context.Context, tenantID, email, status string) error {
	if f.updateStatusErr != nil {
		return f.updateStatusErr
	}
	key := f.userKey(tenantID, email)
	inv, ok := f.invites[key]
	if !ok {
		return users.ErrInviteNotFound
	}
	inv.Status = status
	f.invites[key] = inv
	return nil
}

func setupUsersService(repo users.Repository) *users.Service {
	clock := func() time.Time { return time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC) }
	return users.NewService(repo, clock)
}

func TestUsersService_BootstrapPrimeiroAdmin_Sucesso(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := setupUsersService(repo)
	ctx := context.Background()

	admin, err := svc.BootstrapFirstAdmin(ctx, "tenant-empresa", "admin@empresa.com")
	require.NoError(t, err)
	assert.Equal(t, "admin@empresa.com", admin.Email)
	assert.Equal(t, "admin", admin.Role)
	assert.Equal(t, "tenant-empresa", admin.TenantID)

	count, err := repo.CountUsersByTenant(ctx, "tenant-empresa")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestUsersService_BootstrapPrimeiroAdmin_SeJaExisteRetornaErro(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := setupUsersService(repo)
	ctx := context.Background()

	_, err := svc.BootstrapFirstAdmin(ctx, "tenant-empresa", "admin@empresa.com")
	require.NoError(t, err)

	_, err = svc.BootstrapFirstAdmin(ctx, "tenant-empresa", "outro@empresa.com")
	require.ErrorIs(t, err, users.ErrUserAlreadyExists)
}

func TestUsersService_InviteUser_AdminPodeConvidar(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := setupUsersService(repo)
	ctx := context.Background()

	admin, err := svc.BootstrapFirstAdmin(ctx, "tenant-1", "admin@empresa.com")
	require.NoError(t, err)

	inv, err := svc.InviteUser(ctx, "tenant-1", admin.Role, admin.ID, "membro@empresa.com", "member")
	require.NoError(t, err)
	assert.Equal(t, "membro@empresa.com", inv.Email)
	assert.Equal(t, "member", inv.Role)
	assert.Equal(t, "pending", inv.Status)
}

func TestUsersService_InviteUser_NaoAdminNaoPodeConvidar(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := setupUsersService(repo)
	ctx := context.Background()

	_, err := svc.InviteUser(ctx, "tenant-1", "member", "user-comum", "novo@empresa.com", "member")
	require.ErrorIs(t, err, users.ErrOnlyAdminCanInvite)
}

func TestUsersService_InviteUser_EmailInvalidoOuVazio_RetornaErro(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := setupUsersService(repo)
	ctx := context.Background()

	_, err := svc.InviteUser(ctx, "tenant-1", "admin", "admin-id", "", "member")
	require.ErrorIs(t, err, users.ErrInvalidEmail)

	_, err = svc.InviteUser(ctx, "tenant-1", "admin", "admin-id", "email-sem-arroba", "member")
	require.ErrorIs(t, err, users.ErrInvalidEmail)
}

func TestUsersService_AcceptInvite_CriaUsuarioEAtualizaStatus(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := setupUsersService(repo)
	ctx := context.Background()

	admin, err := svc.BootstrapFirstAdmin(ctx, "tenant-1", "admin@empresa.com")
	require.NoError(t, err)

	_, err = svc.InviteUser(ctx, "tenant-1", admin.Role, admin.ID, "membro@empresa.com", "member")
	require.NoError(t, err)

	u, err := svc.AcceptInvite(ctx, "tenant-1", "membro@empresa.com")
	require.NoError(t, err)
	assert.Equal(t, "membro@empresa.com", u.Email)
	assert.Equal(t, "member", u.Role)

	inv, err := repo.GetInviteByEmail(ctx, "tenant-1", "membro@empresa.com")
	require.NoError(t, err)
	assert.Equal(t, "accepted", inv.Status)
}

func TestUsersService_ListUsersEListInvites_Sucesso(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := setupUsersService(repo)
	ctx := context.Background()

	admin, err := svc.BootstrapFirstAdmin(ctx, "tenant-1", "admin@empresa.com")
	require.NoError(t, err)

	_, err = svc.InviteUser(ctx, "tenant-1", admin.Role, admin.ID, "convidado@empresa.com", "member")
	require.NoError(t, err)

	listaUsers, err := svc.ListUsers(ctx, "tenant-1")
	require.NoError(t, err)
	assert.Len(t, listaUsers, 1)

	listaInvites, err := svc.ListInvites(ctx, "tenant-1")
	require.NoError(t, err)
	assert.Len(t, listaInvites, 1)
}

func TestNewService_ClockNil_AdotaRelogioPadrao(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := users.NewService(repo, nil)
	assert.NotNil(t, svc)
}

func TestUsersService_BootstrapFirstAdmin_ErroNoCount(t *testing.T) {
	repo := newFakeUsersRepository()
	repo.countErr = assert.AnError
	svc := setupUsersService(repo)

	_, err := svc.BootstrapFirstAdmin(context.Background(), "t1", "adm@corp.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contar usuarios")
}

func TestUsersService_BootstrapFirstAdmin_ErroNoCreate(t *testing.T) {
	repo := newFakeUsersRepository()
	repo.createErr = assert.AnError
	svc := setupUsersService(repo)

	_, err := svc.BootstrapFirstAdmin(context.Background(), "t1", "adm@corp.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "criar admin")
}

func TestUsersService_InviteUser_ErroNoCreateInvite(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := setupUsersService(repo)
	ctx := context.Background()

	_, err := svc.InviteUser(ctx, "t1", "admin", "admin-id", "user@corp.com", "member")
	require.NoError(t, err)

	_, err = svc.InviteUser(ctx, "t1", "admin", "admin-id", "user@corp.com", "member")
	require.ErrorIs(t, err, users.ErrInviteAlreadyExists)
}

func TestUsersService_InviteUser_UsuarioJaExiste(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := setupUsersService(repo)
	ctx := context.Background()

	_, err := svc.BootstrapFirstAdmin(ctx, "t1", "user@corp.com")
	require.NoError(t, err)

	_, err = svc.InviteUser(ctx, "t1", "admin", "admin-id", "user@corp.com", "member")
	require.ErrorIs(t, err, users.ErrUserAlreadyExists)
}

func TestUsersService_AcceptInvite_EmailInvalidoOuNaoEncontrado(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := setupUsersService(repo)
	ctx := context.Background()

	_, err := svc.AcceptInvite(ctx, "t1", "invalido")
	require.ErrorIs(t, err, users.ErrInvalidEmail)

	_, err = svc.AcceptInvite(ctx, "t1", "inexistente@corp.com")
	require.ErrorIs(t, err, users.ErrInviteNotFound)
}

func TestUsersService_AcceptInvite_ErroAoCriarUsuarioOuAtualizarStatus(t *testing.T) {
	repo := newFakeUsersRepository()
	svc := setupUsersService(repo)
	ctx := context.Background()

	_, err := svc.InviteUser(ctx, "t1", "admin", "admin-id", "user@corp.com", "member")
	require.NoError(t, err)

	repo.createErr = assert.AnError
	_, err = svc.AcceptInvite(ctx, "t1", "user@corp.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "criar usuario por convite")

	repo.createErr = nil
	repo.updateStatusErr = assert.AnError
	_, err = svc.AcceptInvite(ctx, "t1", "user@corp.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "atualizar convite")
}
