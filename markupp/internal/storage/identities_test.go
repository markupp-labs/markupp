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

// externalAccountParams é uma segunda conta, cadastrada pelo provedor github sem senha.
func externalAccountParams(emailVerified bool) auth.CreateAccountParams {
	p := sampleAccountParams()
	p.User.ID, p.User.Email = "user-id-2", "ana@exemplo.org"
	p.Identity = auth.UserIdentity{ID: "identity-id-2", UserID: "user-id-2", Provider: "github", ProviderUserID: "42", CreatedAt: p.User.CreatedAt}
	p.InitialVaultID, p.AuditEventID = "vault-id-2", "audit-id-2"
	p.EmailVerified = emailVerified
	return p
}

// withFirstAccount cria a primeira conta da instalação, a do sampleAccountParams.
func withFirstAccount(t *testing.T, repo *storage.PostgresAuthRepository) {
	t.Helper()
	_, err := repo.CreateAccountTx(context.Background(), sampleAccountParams())
	require.NoError(t, err)
}

func linkEvent(id string) auth.AuditEventRecord {
	return auth.AuditEventRecord{ID: id, ActorID: "user-id-1", Action: "auth.identity_link", Target: "github", Result: "success",
		OccurredAt: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
}

func TestPostgresAuthRepo_CreateAccountTx_EmailProvadoPeloProvedor_NasceVerificado(t *testing.T) {
	repo := storage.NewPostgresAuthRepository(setupTestDB(t))
	withFirstAccount(t, repo)

	_, err := repo.CreateAccountTx(context.Background(), externalAccountParams(true))
	require.NoError(t, err)

	user, err := repo.GetUserByID(context.Background(), "user-id-2")
	require.NoError(t, err)
	assert.True(t, user.EmailVerified)
}

func TestPostgresAuthRepo_CreateAccountTx_AutoRegistroSemProva_NasceNaoVerificado(t *testing.T) {
	repo := storage.NewPostgresAuthRepository(setupTestDB(t))
	withFirstAccount(t, repo)

	_, err := repo.CreateAccountTx(context.Background(), externalAccountParams(false))
	require.NoError(t, err)

	user, err := repo.GetUserByID(context.Background(), "user-id-2")
	require.NoError(t, err)
	assert.False(t, user.EmailVerified)
}

func TestPostgresAuthRepo_ListUserIdentities_DevolveTodasNaOrdemDeCriacao(t *testing.T) {
	repo := storage.NewPostgresAuthRepository(setupTestDB(t))
	ctx := context.Background()
	withFirstAccount(t, repo)
	github := auth.UserIdentity{ID: "identity-gh", UserID: "user-id-1", Provider: "github", ProviderUserID: "42",
		CreatedAt: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
	require.NoError(t, repo.LinkIdentityTx(ctx, github, linkEvent("audit-link")))

	identities, err := repo.ListUserIdentities(ctx, "user-id-1")

	require.NoError(t, err)
	require.Len(t, identities, 2)
	assert.Equal(t, "local", identities[0].Provider)
	assert.Equal(t, github, identities[1])
}

func TestPostgresAuthRepo_LinkIdentityTx_IdentidadeJaUsada_Recusa(t *testing.T) {
	repo := storage.NewPostgresAuthRepository(setupTestDB(t))
	ctx := context.Background()
	withFirstAccount(t, repo)
	_, err := repo.CreateAccountTx(ctx, externalAccountParams(true))
	require.NoError(t, err)
	repetida := auth.UserIdentity{ID: "identity-gh-2", UserID: "user-id-1", Provider: "github", ProviderUserID: "42", CreatedAt: time.Now()}

	err = repo.LinkIdentityTx(ctx, repetida, linkEvent("audit-link"))

	require.ErrorIs(t, err, auth.ErrIdentityAlreadyLinked)
}

func TestPostgresAuthRepo_UnlinkIdentityTx_ComOutraForma_Apaga(t *testing.T) {
	repo := storage.NewPostgresAuthRepository(setupTestDB(t))
	ctx := context.Background()
	withFirstAccount(t, repo)
	github := auth.UserIdentity{ID: "identity-gh", UserID: "user-id-1", Provider: "github", ProviderUserID: "42", CreatedAt: time.Now()}
	require.NoError(t, repo.LinkIdentityTx(ctx, github, linkEvent("audit-link")))

	err := repo.UnlinkIdentityTx(ctx, "user-id-1", "identity-gh", linkEvent("audit-unlink"))

	require.NoError(t, err)
	_, err = repo.GetUserIdentity(ctx, "github", "42")
	require.ErrorIs(t, err, auth.ErrIdentityNotFound)
}

func TestPostgresAuthRepo_UnlinkIdentityTx_UnicaIdentidade_Recusa(t *testing.T) {
	repo := storage.NewPostgresAuthRepository(setupTestDB(t))
	ctx := context.Background()
	withFirstAccount(t, repo)
	_, err := repo.CreateAccountTx(ctx, externalAccountParams(true))
	require.NoError(t, err)

	err = repo.UnlinkIdentityTx(ctx, "user-id-2", "identity-id-2", linkEvent("audit-unlink"))

	require.ErrorIs(t, err, auth.ErrLastIdentity)
	_, err = repo.GetUserIdentity(ctx, "github", "42")
	require.NoError(t, err)
}

func TestPostgresAuthRepo_UnlinkIdentityTx_IdentidadeDeOutraConta_NaoAcha(t *testing.T) {
	repo := storage.NewPostgresAuthRepository(setupTestDB(t))
	ctx := context.Background()
	withFirstAccount(t, repo)
	_, err := repo.CreateAccountTx(ctx, externalAccountParams(true))
	require.NoError(t, err)

	err = repo.UnlinkIdentityTx(ctx, "user-id-1", "identity-id-2", linkEvent("audit-unlink"))

	require.ErrorIs(t, err, auth.ErrIdentityNotFound)
}
