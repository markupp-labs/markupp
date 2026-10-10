package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

func sampleAccountParams() auth.CreateAccountParams {
	now := time.Date(2026, 10, 4, 15, 0, 0, 0, time.UTC)
	return auth.CreateAccountParams{
		User: auth.User{
			ID:        "user-id-1",
			TenantID:  "tenant-id-1",
			Email:     "admin@markupp.dev",
			Role:      "admin",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Identity: auth.UserIdentity{
			ID:             "identity-id-1",
			UserID:         "user-id-1",
			Provider:       "local",
			ProviderUserID: "admin@markupp.dev",
			PasswordHash:   "hash-bcrypt-seguro", // #nosec G101 -- hash ficticio para teste
			CreatedAt:      now,
		},
		InitialVaultID:   "vault-id-1",
		InitialVaultName: "Principal",
		AuditEvent: auth.AuditEventRecord{
			ID:         "audit-id-1",
			TenantID:   "tenant-id-1",
			ActorID:    "user-id-1",
			Action:     "auth.register",
			Target:     "admin@markupp.dev",
			Result:     "success",
			OccurredAt: now,
		},
	}
}

func TestPostgresAuthRepo_CreateAccountTx_GravaTodosOsRegistros(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	countBefore, err := repo.CountUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), countBefore)

	p := sampleAccountParams()
	err = repo.CreateAccountTx(ctx, p)
	require.NoError(t, err)

	countAfter, err := repo.CountUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), countAfter)

	user, err := repo.GetUserByID(ctx, p.User.ID)
	require.NoError(t, err)
	assert.Equal(t, p.User.Email, user.Email)
	assert.Equal(t, p.User.Role, user.Role)

	identity, err := repo.GetUserIdentity(ctx, p.Identity.Provider, p.Identity.ProviderUserID)
	require.NoError(t, err)
	assert.Equal(t, p.User.ID, identity.UserID)
	assert.Equal(t, p.Identity.PasswordHash, identity.PasswordHash)
}

func TestPostgresAuthRepo_CreateAccountTx_EmailDuplicadoRetornaErro(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	p1 := sampleAccountParams()
	require.NoError(t, repo.CreateAccountTx(ctx, p1))

	p2 := sampleAccountParams()
	p2.User.ID = "user-id-2"
	p2.Identity.ID = "identity-id-2"
	p2.InitialVaultID = "vault-id-2"
	p2.AuditEvent.ID = "audit-id-2"

	err := repo.CreateAccountTx(ctx, p2)
	require.ErrorIs(t, err, auth.ErrUserAlreadyExists)
}

func TestPostgresAuthRepo_GetUserByEmail_RetornaUsuarioCorreto(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	p := sampleAccountParams()
	require.NoError(t, repo.CreateAccountTx(ctx, p))

	found, err := repo.GetUserByEmail(ctx, p.User.TenantID, p.User.Email)
	require.NoError(t, err)
	assert.Equal(t, p.User.ID, found.ID)

	_, err = repo.GetUserByEmail(ctx, p.User.TenantID, "inexistente@markupp.dev")
	require.ErrorIs(t, err, auth.ErrUserNotFound)
}

func TestPostgresAuthRepo_RefreshToken_FluxoCompleto(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	p := sampleAccountParams()
	require.NoError(t, repo.CreateAccountTx(ctx, p))

	now := time.Now().UTC().Truncate(time.Millisecond)
	record := auth.RefreshTokenRecord{
		TokenHash: "hash-token-12345",
		UserID:    p.User.ID,
		ExpiresAt: now.Add(24 * time.Hour),
		Revoked:   false,
		CreatedAt: now,
	}
	require.NoError(t, repo.SaveRefreshToken(ctx, record))

	retrieved, err := repo.GetRefreshToken(ctx, record.TokenHash)
	require.NoError(t, err)
	assert.Equal(t, record.UserID, retrieved.UserID)
	assert.False(t, retrieved.Revoked)

	require.NoError(t, repo.RevokeRefreshToken(ctx, record.TokenHash))

	revoked, err := repo.GetRefreshToken(ctx, record.TokenHash)
	require.NoError(t, err)
	assert.True(t, revoked.Revoked)
}

func TestPostgresAuthRepo_RecordAuditEvent_GravaSucesso(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	event := auth.AuditEventRecord{
		ID:         "audit-test-event-1",
		TenantID:   "tenant-1",
		ActorID:    "actor-1",
		Action:     "auth.login",
		Target:     "actor-1@markupp.dev",
		Result:     "success",
		OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	require.NoError(t, repo.RecordAuditEvent(ctx, event))
}

func TestPostgresAuthRepo_GetUserByID_Inexistente_RetornaErro(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	_, err := repo.GetUserByID(ctx, "usuario-inexistente")
	require.ErrorIs(t, err, auth.ErrUserNotFound)
}

func TestPostgresAuthRepo_GetUserIdentity_Inexistente_RetornaErro(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	_, err := repo.GetUserIdentity(ctx, "local", "inexistente@markupp.dev")
	require.ErrorIs(t, err, auth.ErrIdentityNotFound)
}

func TestPostgresAuthRepo_GetRefreshToken_Inexistente_RetornaErro(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	_, err := repo.GetRefreshToken(ctx, "hash-inexistente")
	require.ErrorIs(t, err, auth.ErrRefreshTokenExpired)
}

func TestPostgresAuthRepo_RevokeRefreshToken_Inexistente_RetornaErro(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	err := repo.RevokeRefreshToken(ctx, "hash-inexistente")
	require.ErrorIs(t, err, auth.ErrRefreshTokenExpired)
}
