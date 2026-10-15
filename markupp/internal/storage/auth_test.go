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
		InitialVaultID:          "vault-id-1",
		InitialVaultName:        "Principal",
		AllowMemberRegistration: true,
		AuditEventID:            "audit-id-1",
	}
}

func TestPostgresAuthRepo_CreateAccountTx_GravaTodosOsRegistros(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	p := sampleAccountParams()
	user, err := repo.CreateAccountTx(ctx, p)
	require.NoError(t, err)
	assert.Equal(t, "admin", user.Role, "primeiro usuario criado deve ser admin")

	found, err := repo.GetUserByID(ctx, p.User.ID)
	require.NoError(t, err)
	assert.Equal(t, p.User.Email, found.Email)
	assert.Equal(t, "admin", found.Role)

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
	_, err := repo.CreateAccountTx(ctx, p1)
	require.NoError(t, err)

	p2 := sampleAccountParams()
	p2.User.ID = "user-id-2"
	p2.Identity.ID = "identity-id-2"
	p2.InitialVaultID = "vault-id-2"
	p2.AuditEventID = "audit-id-2"

	_, err = repo.CreateAccountTx(ctx, p2)
	require.ErrorIs(t, err, auth.ErrUserAlreadyExists)
}

func TestPostgresAuthRepo_RefreshToken_FluxoCompleto(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	p := sampleAccountParams()
	_, err := repo.CreateAccountTx(ctx, p)
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Millisecond)
	record := auth.RefreshTokenRecord{
		TokenHash: "hash-token-12345",
		UserID:    p.User.ID,
		ExpiresAt: now.Add(24 * time.Hour),
		Revoked:   false,
		CreatedAt: now,
	}
	event := auth.AuditEventRecord{
		ID:         "audit-login-1",
		TenantID:   p.User.TenantID,
		ActorID:    p.User.ID,
		Action:     "auth.login",
		Target:     p.User.Email,
		Result:     "success",
		OccurredAt: now,
	}
	require.NoError(t, repo.IssueSessionTx(ctx, record, event))

	retrieved, err := repo.GetRefreshToken(ctx, record.TokenHash)
	require.NoError(t, err)
	assert.Equal(t, record.UserID, retrieved.UserID)
	assert.False(t, retrieved.Revoked)

	consumedUserID, err := repo.ConsumeRefreshToken(ctx, record.TokenHash)
	require.NoError(t, err)
	assert.Equal(t, p.User.ID, consumedUserID)

	_, err = repo.ConsumeRefreshToken(ctx, record.TokenHash)
	require.ErrorIs(t, err, auth.ErrRefreshTokenInvalid, "token ja consumido nao pode ser reutilizado")
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
	require.ErrorIs(t, err, auth.ErrRefreshTokenInvalid)
}

func TestPostgresAuthRepo_RevokeRefreshToken_Inexistente_RetornaErro(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	err := repo.RevokeRefreshToken(ctx, "hash-inexistente")
	require.ErrorIs(t, err, auth.ErrRefreshTokenInvalid)
}

func TestPostgresAuthRepo_RevokeAllUserRefreshTokens_RevogaTokensDoUsuario(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresAuthRepository(db)
	ctx := context.Background()

	p := sampleAccountParams()
	_, err := repo.CreateAccountTx(ctx, p)
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Millisecond)
	rec1 := auth.RefreshTokenRecord{
		TokenHash: "hash-token-a",
		UserID:    p.User.ID,
		ExpiresAt: now.Add(24 * time.Hour),
		Revoked:   false,
		CreatedAt: now,
	}
	event1 := auth.AuditEventRecord{
		ID:         "audit-1",
		TenantID:   p.User.TenantID,
		ActorID:    p.User.ID,
		Action:     "auth.login",
		Target:     p.User.Email,
		Result:     "success",
		OccurredAt: now,
	}
	require.NoError(t, repo.IssueSessionTx(ctx, rec1, event1))

	require.NoError(t, repo.RevokeAllUserRefreshTokens(ctx, p.User.ID))

	_, err = repo.ConsumeRefreshToken(ctx, rec1.TokenHash)
	require.ErrorIs(t, err, auth.ErrRefreshTokenInvalid)
}
