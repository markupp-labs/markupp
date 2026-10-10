package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/users"
)

func TestPostgresUserRepo_CreateUserECount_Sucesso(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresUserRepository(db)
	ctx := context.Background()

	count, err := repo.CountUsersByTenant(ctx, "tenant-test")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	now := time.Now().UTC().Truncate(time.Millisecond)
	u := users.User{
		ID:        "user-1",
		TenantID:  "tenant-test",
		Email:     "admin@test.com",
		Role:      "admin",
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, repo.CreateUser(ctx, u))

	count, err = repo.CountUsersByTenant(ctx, "tenant-test")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	byID, err := repo.GetUserByID(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, u.Email, byID.Email)
	assert.Equal(t, u.Role, byID.Role)

	byEmail, err := repo.GetUserByEmail(ctx, "tenant-test", "admin@test.com")
	require.NoError(t, err)
	assert.Equal(t, u.ID, byEmail.ID)
}

func TestPostgresUserRepo_CreateUser_DuplicadoRetornaErro(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresUserRepository(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	u1 := users.User{
		ID:        "user-1",
		TenantID:  "tenant-test",
		Email:     "admin@test.com",
		Role:      "admin",
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, repo.CreateUser(ctx, u1))

	u2 := users.User{
		ID:        "user-2",
		TenantID:  "tenant-test",
		Email:     "admin@test.com",
		Role:      "member",
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.ErrorIs(t, repo.CreateUser(ctx, u2), users.ErrUserAlreadyExists)
}

func TestPostgresUserRepo_GetUser_NaoEncontrado_RetornaErro(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetUserByID(ctx, "inexistente")
	require.ErrorIs(t, err, users.ErrUserNotFound)

	_, err = repo.GetUserByEmail(ctx, "tenant-test", "inexistente@test.com")
	require.ErrorIs(t, err, users.ErrUserNotFound)
}

func TestPostgresUserRepo_ListUsers_IsolaPorTenant(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresUserRepository(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	require.NoError(t, repo.CreateUser(ctx, users.User{
		ID: "u1", TenantID: "tenant-a", Email: "a@test.com", Role: "admin", CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, repo.CreateUser(ctx, users.User{
		ID: "u2", TenantID: "tenant-b", Email: "b@test.com", Role: "admin", CreatedAt: now, UpdatedAt: now,
	}))

	listA, err := repo.ListUsers(ctx, "tenant-a")
	require.NoError(t, err)
	require.Len(t, listA, 1)
	assert.Equal(t, "u1", listA[0].ID)

	listB, err := repo.ListUsers(ctx, "tenant-b")
	require.NoError(t, err)
	require.Len(t, listB, 1)
	assert.Equal(t, "u2", listB[0].ID)
}

func TestPostgresUserRepo_Invites_FluxoCompleto(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresUserRepository(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	require.NoError(t, repo.CreateUser(ctx, users.User{
		ID: "admin-1", TenantID: "tenant-inv", Email: "admin@inv.com", Role: "admin", CreatedAt: now, UpdatedAt: now,
	}))

	inv := users.UserInvite{
		ID:        "inv-1",
		TenantID:  "tenant-inv",
		Email:     "convidado@inv.com",
		Role:      "member",
		InvitedBy: "admin-1",
		Status:    "pending",
		CreatedAt: now,
	}
	require.NoError(t, repo.CreateInvite(ctx, inv))

	// Nao permite convite duplicado no mesmo tenant
	require.ErrorIs(t, repo.CreateInvite(ctx, inv), users.ErrInviteAlreadyExists)

	fetched, err := repo.GetInviteByEmail(ctx, "tenant-inv", "convidado@inv.com")
	require.NoError(t, err)
	assert.Equal(t, "pending", fetched.Status)

	list, err := repo.ListInvites(ctx, "tenant-inv")
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, repo.UpdateInviteStatus(ctx, "tenant-inv", "convidado@inv.com", "accepted"))
	fetched, err = repo.GetInviteByEmail(ctx, "tenant-inv", "convidado@inv.com")
	require.NoError(t, err)
	assert.Equal(t, "accepted", fetched.Status)

	// Inexistente
	_, err = repo.GetInviteByEmail(ctx, "tenant-inv", "naoexiste@inv.com")
	require.ErrorIs(t, err, users.ErrInviteNotFound)

	err = repo.UpdateInviteStatus(ctx, "tenant-inv", "naoexiste@inv.com", "revoked")
	require.ErrorIs(t, err, users.ErrInviteNotFound)
}
