package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

type fakeHasher struct{}

func (f *fakeHasher) Hash(password string) (string, error) {
	return "hash:" + password, nil
}

func (f *fakeHasher) Compare(hash, password string) error {
	if hash != "hash:"+password {
		return errors.New("senha nao confere")
	}
	return nil
}

type fakeRepository struct {
	users         map[string]auth.User
	usersByEmail  map[string]auth.User
	identities    map[string]auth.UserIdentity
	refreshTokens map[string]auth.RefreshTokenRecord
	auditEvents   []auth.AuditEventRecord
	createdVaults []string
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		users:         make(map[string]auth.User),
		usersByEmail:  make(map[string]auth.User),
		identities:    make(map[string]auth.UserIdentity),
		refreshTokens: make(map[string]auth.RefreshTokenRecord),
	}
}

func (f *fakeRepository) CountUsers(ctx context.Context) (int64, error) {
	return int64(len(f.users)), nil
}

func (f *fakeRepository) CreateAccountTx(ctx context.Context, p auth.CreateAccountParams) error {
	key := p.User.TenantID + ":" + p.User.Email
	if _, exists := f.usersByEmail[key]; exists {
		return auth.ErrUserAlreadyExists
	}
	f.users[p.User.ID] = p.User
	f.usersByEmail[key] = p.User
	f.identities[p.Identity.Provider+":"+p.Identity.ProviderUserID] = p.Identity
	f.createdVaults = append(f.createdVaults, p.InitialVaultID)
	f.auditEvents = append(f.auditEvents, p.AuditEvent)
	return nil
}

func (f *fakeRepository) GetUserByID(ctx context.Context, id string) (auth.User, error) {
	u, ok := f.users[id]
	if !ok {
		return auth.User{}, auth.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeRepository) GetUserByEmail(ctx context.Context, tenantID, email string) (auth.User, error) {
	u, ok := f.usersByEmail[tenantID+":"+email]
	if !ok {
		return auth.User{}, auth.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeRepository) GetUserIdentity(ctx context.Context, provider, providerUserID string) (auth.UserIdentity, error) {
	id, ok := f.identities[provider+":"+providerUserID]
	if !ok {
		return auth.UserIdentity{}, auth.ErrIdentityNotFound
	}
	return id, nil
}

func (f *fakeRepository) SaveRefreshToken(ctx context.Context, rec auth.RefreshTokenRecord) error {
	f.refreshTokens[rec.TokenHash] = rec
	return nil
}

func (f *fakeRepository) GetRefreshToken(ctx context.Context, tokenHash string) (auth.RefreshTokenRecord, error) {
	rec, ok := f.refreshTokens[tokenHash]
	if !ok {
		return auth.RefreshTokenRecord{}, auth.ErrRefreshTokenExpired
	}
	return rec, nil
}

func (f *fakeRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	rec, ok := f.refreshTokens[tokenHash]
	if !ok {
		return auth.ErrRefreshTokenExpired
	}
	rec.Revoked = true
	f.refreshTokens[tokenHash] = rec
	return nil
}

func (f *fakeRepository) RecordAuditEvent(ctx context.Context, event auth.AuditEventRecord) error {
	f.auditEvents = append(f.auditEvents, event)
	return nil
}

func setupAuthService(repo auth.Repository, registrationEnabled bool) *auth.Service {
	clock := func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC) }
	codec := auth.NewTokenCodec("segredo-para-testes-de-servico", 15*time.Minute, clock)
	hasher := &fakeHasher{}
	cfg := auth.ServiceConfig{
		DefaultTenantID:         "default",
		RegistrationEnabled:     registrationEnabled,
		LocalEnabled:            true,
		RefreshExpirationPeriod: 7 * 24 * time.Hour,
	}
	return auth.NewService(repo, codec, hasher, cfg, clock)
}

func TestService_PrimeiroUsuario_PromovidoAAdminECriaCofre(t *testing.T) {
	repo := newFakeRepository()
	svc := setupAuthService(repo, false)
	ctx := context.Background()

	pair, err := svc.LoginLocal(ctx, "primeiro@markupp.dev", "senhaForte123")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)

	count, err := repo.CountUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	user, err := repo.GetUserByEmail(ctx, "default", "primeiro@markupp.dev")
	require.NoError(t, err)
	assert.Equal(t, "admin", user.Role)
	assert.Len(t, repo.createdVaults, 1)
	assert.NotEmpty(t, repo.auditEvents)
}

func TestService_LoginLocal_CredenciaisCorretas_Sucesso(t *testing.T) {
	repo := newFakeRepository()
	svc := setupAuthService(repo, false)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	pair, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
}

func TestService_LoginLocal_SenhaIncorreta_Falha(t *testing.T) {
	repo := newFakeRepository()
	svc := setupAuthService(repo, false)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senhaCorreta")
	require.NoError(t, err)

	_, err = svc.LoginLocal(ctx, "admin@markupp.dev", "senhaErrada")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)

	ultimoEvento := repo.auditEvents[len(repo.auditEvents)-1]
	assert.Equal(t, "failure", ultimoEvento.Result)
}

func TestService_LoginLocal_UsuarioNaoExisteEAutoRegistroDesativado_Rejeita(t *testing.T) {
	repo := newFakeRepository()
	svc := setupAuthService(repo, false)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	_, err = svc.LoginLocal(ctx, "outro@markupp.dev", "senhaQualquer")
	require.ErrorIs(t, err, auth.ErrRegistrationDisabled)
}

func TestService_LoginLocal_EmailOuSenhaVazios_Rejeita(t *testing.T) {
	repo := newFakeRepository()
	svc := setupAuthService(repo, false)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "", "senha")
	require.ErrorIs(t, err, auth.ErrEmptyEmail)

	_, err = svc.LoginLocal(ctx, "teste@markupp.dev", "")
	require.ErrorIs(t, err, auth.ErrEmptyPassword)
}

func TestService_RefreshToken_Valido_RenovaSessaoERevogaAnterior(t *testing.T) {
	repo := newFakeRepository()
	svc := setupAuthService(repo, false)
	ctx := context.Background()

	pair1, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	pair2, err := svc.RefreshToken(ctx, pair1.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, pair2.AccessToken)
	assert.NotEmpty(t, pair2.RefreshToken)
	assert.NotEqual(t, pair1.RefreshToken, pair2.RefreshToken)

	_, err = svc.RefreshToken(ctx, pair1.RefreshToken)
	require.ErrorIs(t, err, auth.ErrRefreshTokenRevoked)
}

func TestService_RevokeSession_Sucesso(t *testing.T) {
	repo := newFakeRepository()
	svc := setupAuthService(repo, false)
	ctx := context.Background()

	pair, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	err = svc.RevokeSession(ctx, pair.RefreshToken)
	require.NoError(t, err)

	_, err = svc.RefreshToken(ctx, pair.RefreshToken)
	require.ErrorIs(t, err, auth.ErrRefreshTokenRevoked)
}

func TestService_LoginLocal_UsuarioNaoExisteEAutoRegistroAtivado_CriaMembro(t *testing.T) {
	repo := newFakeRepository()
	svc := setupAuthService(repo, true)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	pair, err := svc.LoginLocal(ctx, "novo.membro@markupp.dev", "senhaMembro123")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)

	user, err := repo.GetUserByEmail(ctx, "default", "novo.membro@markupp.dev")
	require.NoError(t, err)
	assert.Equal(t, "member", user.Role)
}

func TestService_GetAccount_Sucesso(t *testing.T) {
	repo := newFakeRepository()
	svc := setupAuthService(repo, false)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	admin, err := repo.GetUserByEmail(ctx, "default", "admin@markupp.dev")
	require.NoError(t, err)

	conta, err := svc.GetAccount(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, admin.ID, conta.ID)
	assert.Equal(t, "admin@markupp.dev", conta.Email)
}

func TestService_GetAccount_Inexistente_RetornaErro(t *testing.T) {
	repo := newFakeRepository()
	svc := setupAuthService(repo, false)
	ctx := context.Background()

	_, err := svc.GetAccount(ctx, "usuario-inexistente")
	require.ErrorIs(t, err, auth.ErrUserNotFound)
}

func TestService_RefreshToken_Expirado_Falha(t *testing.T) {
	repo := newFakeRepository()
	agora := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return agora }
	codec := auth.NewTokenCodec("segredo-para-testes-de-servico", 15*time.Minute, clock)
	hasher := &fakeHasher{}
	cfg := auth.ServiceConfig{
		DefaultTenantID:         "default",
		RegistrationEnabled:     false,
		LocalEnabled:            true,
		RefreshExpirationPeriod: 1 * time.Hour,
	}
	svc := auth.NewService(repo, codec, hasher, cfg, clock)
	ctx := context.Background()

	pair, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	agora = agora.Add(2 * time.Hour)
	_, err = svc.RefreshToken(ctx, pair.RefreshToken)
	require.ErrorIs(t, err, auth.ErrRefreshTokenExpired)
}

func TestService_RefreshToken_UsuarioInexistente_Falha(t *testing.T) {
	repo := newFakeRepository()
	svc := setupAuthService(repo, false)
	ctx := context.Background()

	pair, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	delete(repo.users, "user-1")
	for k := range repo.users {
		delete(repo.users, k)
	}

	_, err = svc.RefreshToken(ctx, pair.RefreshToken)
	require.ErrorIs(t, err, auth.ErrUserNotFound)
}
