package auth_test

import (
	"context"
	"errors"
	"sort"
	"strings"
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
	pendingInvites map[string]auth.Invite
	acceptedWith   auth.AcceptInviteParams

	memberships   map[string]auth.Membership
	createdTenant auth.CreateTenantParams

	pendingForUser   []auth.PendingInvite
	acceptedByID     auth.AcceptInviteByIDParams
	acceptByIDResult auth.Membership
	acceptByIDErr    error
	declined         auth.DeclineInviteParams

	clock         func() time.Time
	users         map[string]auth.User
	identities    map[string]auth.UserIdentity
	refreshTokens map[string]auth.RefreshTokenRecord
	auditEvents   []auth.AuditEventRecord
	createdVaults []string
	lastAccount   auth.CreateAccountParams
}

func newFakeRepository(clock func() time.Time) *fakeRepository {
	return &fakeRepository{
		clock:         clock,
		users:         make(map[string]auth.User),
		memberships:   make(map[string]auth.Membership),
		identities:    make(map[string]auth.UserIdentity),
		refreshTokens: make(map[string]auth.RefreshTokenRecord),
	}
}

func (f *fakeRepository) now() time.Time {
	if f.clock != nil {
		return f.clock()
	}
	return time.Now()
}

func (f *fakeRepository) CreateAccountTx(ctx context.Context, p auth.CreateAccountParams) (auth.User, error) {
	f.lastAccount = p
	for _, u := range f.users {
		if u.Email == p.User.Email {
			return auth.User{}, auth.ErrUserAlreadyExists
		}
	}
	action := "auth.register"
	tenant := ""
	if len(f.users) == 0 {
		action, tenant = "auth.register_admin", p.BootstrapTenantID
		f.memberships[tenant+":"+p.User.ID] = auth.Membership{TenantID: tenant, TenantName: tenant, Role: "owner"}
	} else if !p.AllowMemberRegistration {
		return auth.User{}, auth.ErrRegistrationDisabled
	}
	user := p.User
	f.users[user.ID] = user
	f.identities[p.Identity.Provider+":"+p.Identity.ProviderUserID] = p.Identity
	f.createdVaults = append(f.createdVaults, p.InitialVaultID)
	f.auditEvents = append(f.auditEvents, auth.AuditEventRecord{
		ID:         p.AuditEventID,
		TenantID:   tenant,
		ActorID:    user.ID,
		Action:     action,
		Target:     user.Email,
		Result:     "success",
		OccurredAt: user.CreatedAt,
	})
	return user, nil
}

func (f *fakeRepository) AcceptInviteTx(ctx context.Context, p auth.AcceptInviteParams) (auth.User, error) {
	inv, ok := f.pendingInvites[p.TokenHash]
	if !ok {
		return auth.User{}, auth.ErrInviteInvalid
	}
	delete(f.pendingInvites, p.TokenHash)
	f.acceptedWith = p
	user := auth.User{ID: p.UserID, Email: inv.Email, CreatedAt: p.Now, UpdatedAt: p.Now}
	f.users[user.ID] = user
	f.memberships[inv.TenantID+":"+user.ID] = auth.Membership{TenantID: inv.TenantID, TenantName: inv.TenantID, Role: inv.Role}
	return user, nil
}

func (f *fakeRepository) GetMembership(ctx context.Context, userID, tenantID string) (auth.Membership, error) {
	m, ok := f.memberships[tenantID+":"+userID]
	if !ok {
		return auth.Membership{}, auth.ErrMembershipNotFound
	}
	return m, nil
}

func (f *fakeRepository) ListMemberships(ctx context.Context, userID string) ([]auth.Membership, error) {
	var out []auth.Membership
	for chave, m := range f.memberships {
		if strings.HasSuffix(chave, ":"+userID) {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TenantID < out[j].TenantID })
	return out, nil
}

func (f *fakeRepository) ListTenantSummaries(ctx context.Context, userID string) ([]auth.TenantSummary, error) {
	memberships, _ := f.ListMemberships(ctx, userID)
	out := make([]auth.TenantSummary, 0, len(memberships))
	for _, m := range memberships {
		out = append(out, auth.TenantSummary{Membership: m})
	}
	return out, nil
}

func (f *fakeRepository) GetUserByID(ctx context.Context, id string) (auth.User, error) {
	u, ok := f.users[id]
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

func (f *fakeRepository) ConsumeRefreshToken(ctx context.Context, tokenHash string) (string, error) {
	rec, ok := f.refreshTokens[tokenHash]
	if !ok || rec.Revoked || rec.ExpiresAt.Before(f.now()) {
		return "", auth.ErrRefreshTokenInvalid
	}
	rec.Revoked = true
	f.refreshTokens[tokenHash] = rec
	return rec.UserID, nil
}

func (f *fakeRepository) IssueSessionTx(ctx context.Context, record auth.RefreshTokenRecord, event auth.AuditEventRecord) error {
	f.refreshTokens[record.TokenHash] = record
	f.auditEvents = append(f.auditEvents, event)
	return nil
}

func (f *fakeRepository) GetRefreshToken(ctx context.Context, tokenHash string) (auth.RefreshTokenRecord, error) {
	rec, ok := f.refreshTokens[tokenHash]
	if !ok {
		return auth.RefreshTokenRecord{}, auth.ErrRefreshTokenInvalid
	}
	return rec, nil
}

func (f *fakeRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	rec, ok := f.refreshTokens[tokenHash]
	if !ok || rec.Revoked {
		return auth.ErrRefreshTokenInvalid
	}
	rec.Revoked = true
	f.refreshTokens[tokenHash] = rec
	return nil
}

func (f *fakeRepository) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	for k, v := range f.refreshTokens {
		if v.UserID == userID {
			v.Revoked = true
			f.refreshTokens[k] = v
		}
	}
	return nil
}

func (f *fakeRepository) RecordAuditEvent(ctx context.Context, event auth.AuditEventRecord) error {
	f.auditEvents = append(f.auditEvents, event)
	return nil
}

func setupAuthService(repo auth.Repository, registrationEnabled bool, clock func() time.Time) *auth.Service {
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

func findUserByEmail(users map[string]auth.User, email string) (auth.User, bool) {
	for _, u := range users {
		if u.Email == email {
			return u, true
		}
	}
	return auth.User{}, false
}

func defaultClock() func() time.Time {
	return func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC) }
}

func TestService_PrimeiroUsuario_PromovidoAAdminECriaCofre(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	pair, err := svc.LoginLocal(ctx, "primeiro@markupp.dev", "senhaForte123")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)

	assert.Len(t, repo.users, 1)
	user, found := findUserByEmail(repo.users, "primeiro@markupp.dev")
	require.True(t, found)
	assert.Equal(t, "owner", repo.memberships["default:"+user.ID].Role)
	assert.Len(t, repo.createdVaults, 1)
	assert.NotEmpty(t, repo.auditEvents)
}

func TestService_LoginLocal_CredenciaisCorretas_Sucesso(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	pair, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
}

func TestService_LoginLocal_SenhaIncorreta_Falha(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senhaCorreta")
	require.NoError(t, err)

	_, err = svc.LoginLocal(ctx, "admin@markupp.dev", "senhaErrada")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)

	ultimoEvento := repo.auditEvents[len(repo.auditEvents)-1]
	assert.Equal(t, "failure", ultimoEvento.Result)
}

func TestService_LoginLocal_UsuarioNaoExisteEAutoRegistroDesativado_Rejeita(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	_, err = svc.LoginLocal(ctx, "outro@markupp.dev", "senhaQualquer")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestService_LoginLocal_EmailOuSenhaVazios_Rejeita(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "", "senha")
	require.ErrorIs(t, err, auth.ErrEmptyEmail)

	_, err = svc.LoginLocal(ctx, "teste@markupp.dev", "")
	require.ErrorIs(t, err, auth.ErrEmptyPassword)
}

func TestService_RefreshToken_Valido_RenovaSessaoERevogaAnterior(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
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
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	pair, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	err = svc.RevokeSession(ctx, pair.RefreshToken)
	require.NoError(t, err)

	_, err = svc.RefreshToken(ctx, pair.RefreshToken)
	require.ErrorIs(t, err, auth.ErrRefreshTokenRevoked)
}

func TestService_LoginLocal_UsuarioNaoExisteEAutoRegistroAtivado_CriaMembro(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, true, clock)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	pair, err := svc.LoginLocal(ctx, "novo.membro@markupp.dev", "senhaMembro123")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)

	user, found := findUserByEmail(repo.users, "novo.membro@markupp.dev")
	require.True(t, found)
	vinculos, err := svc.ListTenants(ctx, user.ID)
	require.NoError(t, err)
	assert.Empty(t, vinculos, "auto-registro cria a conta sem tenant")
}

func TestService_GetAccount_Sucesso(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	user, found := findUserByEmail(repo.users, "admin@markupp.dev")
	require.True(t, found)

	conta, err := svc.GetAccount(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, conta.ID)
	assert.Equal(t, "admin@markupp.dev", conta.Email)
}

func TestService_GetAccount_Inexistente_RetornaErro(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	_, err := svc.GetAccount(ctx, "usuario-inexistente")
	require.ErrorIs(t, err, auth.ErrUserNotFound)
}

func TestService_RefreshToken_Expirado_Falha(t *testing.T) {
	agora := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return agora }
	repo := newFakeRepository(func() time.Time { return agora })
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
	require.ErrorIs(t, err, auth.ErrRefreshTokenInvalid)
}

func TestService_RefreshToken_UsuarioInexistente_Falha(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	pair, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	for k := range repo.users {
		delete(repo.users, k)
	}

	_, err = svc.RefreshToken(ctx, pair.RefreshToken)
	require.ErrorIs(t, err, auth.ErrUserNotFound)
}

func TestService_LoginLocal_SenhaMaiorQue72Bytes_RetornaErro(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	senhaLonga := strings.Repeat("a", 73)
	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", senhaLonga)
	require.ErrorIs(t, err, auth.ErrPasswordTooLong)
}

func TestService_LoginLocal_AutenticacaoLocalDesativada_RetornaErro(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	codec := auth.NewTokenCodec("segredo-para-testes-de-servico", 15*time.Minute, clock)
	hasher := &fakeHasher{}
	cfg := auth.ServiceConfig{
		DefaultTenantID:         "default",
		RegistrationEnabled:     false,
		LocalEnabled:            false,
		RefreshExpirationPeriod: 7 * 24 * time.Hour,
	}
	svc := auth.NewService(repo, codec, hasher, cfg, clock)
	ctx := context.Background()

	_, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.ErrorIs(t, err, auth.ErrLocalAuthDisabled)
}

func TestService_RefreshToken_ReusoDetectado_RevogaTodasSessoesEAudita(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	pair1, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	pair2, err := svc.RefreshToken(ctx, pair1.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, pair2.AccessToken)

	_, err = svc.RefreshToken(ctx, pair1.RefreshToken)
	require.ErrorIs(t, err, auth.ErrRefreshTokenRevoked)

	_, err = svc.RefreshToken(ctx, pair2.RefreshToken)
	require.ErrorIs(t, err, auth.ErrRefreshTokenRevoked)

	var reusoEncontrado bool
	for _, ev := range repo.auditEvents {
		if ev.Action == "auth.refresh_reuse" {
			reusoEncontrado = true
			assert.Equal(t, "failure", ev.Result)
		}
	}
	assert.True(t, reusoEncontrado, "auditoria de reuso de token nao foi gravada")
}

func TestService_LoginLocal_PersistenciaAtomica_GravaSessaoEAuditoria(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := context.Background()

	pair, err := svc.LoginLocal(ctx, "admin@markupp.dev", "senha123")
	require.NoError(t, err)

	hash := auth.HashOpaqueToken(pair.RefreshToken)
	rec, ok := repo.refreshTokens[hash]
	require.True(t, ok)
	assert.False(t, rec.Revoked)

	var loginAudit auth.AuditEventRecord
	for _, ev := range repo.auditEvents {
		if ev.Action == "auth.login" && ev.Result == "success" {
			loginAudit = ev
		}
	}
	assert.NotEmpty(t, loginAudit.ID)
	assert.Equal(t, "admin@markupp.dev", loginAudit.Target)
}

// countingHasher conta as operações bcrypt para comparar o custo de cada caminho do login.
type countingHasher struct {
	fakeHasher
	ops int
}

func (c *countingHasher) Hash(password string) (string, error) {
	c.ops++
	return c.fakeHasher.Hash(password)
}

func (c *countingHasher) Compare(hash, password string) error {
	c.ops++
	return c.fakeHasher.Compare(hash, password)
}

func countLoginHasherOps(t *testing.T, email string) int {
	t.Helper()
	clock := defaultClock()
	repo := newFakeRepository(clock)
	hasher := &countingHasher{}
	codec := auth.NewTokenCodec("segredo-para-testes-de-servico", 15*time.Minute, clock)
	cfg := auth.ServiceConfig{DefaultTenantID: "default", LocalEnabled: true, RefreshExpirationPeriod: time.Hour}
	svc := auth.NewService(repo, codec, hasher, cfg, clock)
	_, err := svc.LoginLocal(context.Background(), "admin@markupp.dev", "senha123")
	require.NoError(t, err)
	hasher.ops = 0
	_, err = svc.LoginLocal(context.Background(), email, "senhaErrada")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
	return hasher.ops
}

func TestService_LoginLocal_EmailInexistente_CustaOMesmoQueSenhaErrada(t *testing.T) {
	assert.Equal(t,
		countLoginHasherOps(t, "admin@markupp.dev"),
		countLoginHasherOps(t, "inexistente@markupp.dev"),
		"email inexistente e senha errada devem custar o mesmo numero de operacoes bcrypt")
}

func TestService_RefreshToken_Reuso_AuditaComoEventoDaConta(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	repo.users["u1"] = auth.User{ID: "u1", Email: "b@markupp.dev"}
	repo.refreshTokens[auth.HashOpaqueToken("velho")] = auth.RefreshTokenRecord{
		TokenHash: auth.HashOpaqueToken("velho"), UserID: "u1", ExpiresAt: clock().Add(time.Hour), Revoked: true,
	}
	svc := setupAuthService(repo, false, clock)

	_, err := svc.RefreshToken(context.Background(), "velho")
	require.ErrorIs(t, err, auth.ErrRefreshTokenRevoked)

	ultimo := repo.auditEvents[len(repo.auditEvents)-1]
	assert.Equal(t, "auth.refresh_reuse", ultimo.Action)
	assert.Empty(t, ultimo.TenantID, "sessao e da conta global, sem tenant")
	assert.Equal(t, "u1", ultimo.ActorID)
}

func repoComConvite(clock func() time.Time, token string) *fakeRepository {
	repo := newFakeRepository(clock)
	repo.pendingInvites = map[string]auth.Invite{
		auth.HashOpaqueToken(token): {TenantID: "t1", Email: "nova@markupp.dev", Role: "member"},
	}
	return repo
}

func TestAcceptInvite_TokenValido_CriaContaDoConviteEAbreSessao(t *testing.T) {
	clock := defaultClock()
	repo := repoComConvite(clock, "token-do-convite")
	svc := setupAuthService(repo, false, clock)

	pair, err := svc.AcceptInvite(context.Background(), "token-do-convite", "senhaNova")

	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.Equal(t, "hash:senhaNova", repo.acceptedWith.PasswordHash)
	assert.NotEmpty(t, repo.acceptedWith.InitialVaultID)
	user := repo.users[repo.acceptedWith.UserID]
	assert.Equal(t, auth.User{ID: user.ID, Email: "nova@markupp.dev", CreatedAt: clock(), UpdatedAt: clock()}, user)
	assert.Equal(t, "member", repo.memberships["t1:"+user.ID].Role)
	_, sessaoAberta := repo.refreshTokens[auth.HashOpaqueToken(pair.RefreshToken)]
	assert.True(t, sessaoAberta)
}

func TestAcceptInvite_TokenDesconhecido_RecusaComErrInviteInvalid(t *testing.T) {
	clock := defaultClock()
	svc := setupAuthService(repoComConvite(clock, "token-do-convite"), false, clock)

	_, err := svc.AcceptInvite(context.Background(), "outro-token", "senhaNova")

	require.ErrorIs(t, err, auth.ErrInviteInvalid)
}

func TestAcceptInvite_SenhaLongaDemais_RecusaAntesDoBanco(t *testing.T) {
	clock := defaultClock()
	repo := repoComConvite(clock, "token-do-convite")
	svc := setupAuthService(repo, false, clock)

	_, err := svc.AcceptInvite(context.Background(), "token-do-convite", strings.Repeat("a", 73))

	require.ErrorIs(t, err, auth.ErrPasswordTooLong)
	assert.Len(t, repo.pendingInvites, 1, "convite nao pode ser consumido por senha invalida")
}

func repoComVinculos(papeis map[string]string) *fakeRepository {
	repo := newFakeRepository(defaultClock())
	for tenantID, papel := range papeis {
		repo.memberships[tenantID+":u1"] = auth.Membership{TenantID: tenantID, TenantName: "Nome " + tenantID, Role: papel}
	}
	return repo
}

func TestResolveTenant_TenantPedidoComVinculo_DevolveOPapelDele(t *testing.T) {
	svc := setupAuthService(repoComVinculos(map[string]string{"t1": "member", "t2": "viewer"}), false, defaultClock())

	m, err := svc.ResolveTenant(context.Background(), "u1", "t2")

	require.NoError(t, err)
	assert.Equal(t, auth.Membership{TenantID: "t2", TenantName: "Nome t2", Role: "viewer"}, m)
}

func TestResolveTenant_TenantPedidoSemVinculo_RecusaComErrMembershipNotFound(t *testing.T) {
	svc := setupAuthService(repoComVinculos(map[string]string{"t1": "owner"}), false, defaultClock())

	_, err := svc.ResolveTenant(context.Background(), "u1", "alheio")

	require.ErrorIs(t, err, auth.ErrMembershipNotFound)
}

func TestResolveTenant_SemPedidoComUmTenant_UsaOUnico(t *testing.T) {
	svc := setupAuthService(repoComVinculos(map[string]string{"t1": "admin"}), false, defaultClock())

	m, err := svc.ResolveTenant(context.Background(), "u1", "")

	require.NoError(t, err)
	assert.Equal(t, "t1", m.TenantID)
}

func TestResolveTenant_SemPedidoComVariosTenants_ExigeEscolha(t *testing.T) {
	svc := setupAuthService(repoComVinculos(map[string]string{"t1": "admin", "t2": "member"}), false, defaultClock())

	_, err := svc.ResolveTenant(context.Background(), "u1", "")

	require.ErrorIs(t, err, auth.ErrTenantRequired)
	assert.Contains(t, err.Error(), "2 tenants")
}

func TestResolveTenant_SemNenhumTenant_RecusaComErrNoTenant(t *testing.T) {
	svc := setupAuthService(repoComVinculos(nil), false, defaultClock())

	_, err := svc.ResolveTenant(context.Background(), "u1", "")

	require.ErrorIs(t, err, auth.ErrNoTenant)
}

func (f *fakeRepository) CreateTenantTx(ctx context.Context, p auth.CreateTenantParams) error {
	f.createdTenant = p
	f.memberships[p.TenantID+":"+p.UserID] = auth.Membership{TenantID: p.TenantID, TenantName: p.Name, Role: "owner"}
	return nil
}

func TestCreateTenant_NomeValido_CriadorViraOwnerComCofre(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)

	m, err := svc.CreateTenant(context.Background(), "u1", "  Acme Ltda ")

	require.NoError(t, err)
	assert.Equal(t, "Acme Ltda", m.TenantName)
	assert.Equal(t, "owner", m.Role)
	assert.NotEmpty(t, m.TenantID)
	assert.Equal(t, m.TenantID, repo.createdTenant.TenantID)
	assert.Equal(t, "u1", repo.createdTenant.UserID)
	assert.NotEmpty(t, repo.createdTenant.VaultID)
	assert.Equal(t, clock(), repo.createdTenant.Now)
}

func TestCreateTenant_NomeVazio_RecusaComErrInvalidTenantName(t *testing.T) {
	svc := setupAuthService(newFakeRepository(defaultClock()), false, defaultClock())

	_, err := svc.CreateTenant(context.Background(), "u1", "   ")

	require.ErrorIs(t, err, auth.ErrInvalidTenantName)
}

func (f *fakeRepository) ListPendingInvites(ctx context.Context, userID string, now time.Time) ([]auth.PendingInvite, error) {
	return f.pendingForUser, nil
}

func (f *fakeRepository) AcceptInviteByIDTx(ctx context.Context, p auth.AcceptInviteByIDParams) (auth.Membership, error) {
	f.acceptedByID = p
	return f.acceptByIDResult, f.acceptByIDErr
}

func (f *fakeRepository) DeclineInviteTx(ctx context.Context, p auth.DeclineInviteParams) error {
	f.declined = p
	return nil
}

func TestAcceptMyInvite_RepassaUsuarioConviteECofreNovo(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	repo.acceptByIDResult = auth.Membership{TenantID: "t1", TenantName: "Acme", Role: "viewer"}
	svc := setupAuthService(repo, false, clock)

	m, err := svc.AcceptMyInvite(context.Background(), "u1", "convite-1")

	require.NoError(t, err)
	assert.Equal(t, "viewer", m.Role)
	assert.Equal(t, "u1", repo.acceptedByID.UserID)
	assert.Equal(t, "convite-1", repo.acceptedByID.InviteID)
	assert.NotEmpty(t, repo.acceptedByID.VaultID)
	assert.Equal(t, clock(), repo.acceptedByID.Now)
}

func TestDeclineMyInvite_RepassaUsuarioEConvite(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)

	require.NoError(t, svc.DeclineMyInvite(context.Background(), "u1", "convite-1"))

	assert.Equal(t, "u1", repo.declined.UserID)
	assert.Equal(t, "convite-1", repo.declined.InviteID)
}

// racingRepository simula o outro cadastro do mesmo email terminando primeiro.
type racingRepository struct {
	*fakeRepository
}

func (r racingRepository) CreateAccountTx(ctx context.Context, p auth.CreateAccountParams) (auth.User, error) {
	return auth.User{}, auth.ErrUserAlreadyExists
}

func TestLoginLocal_CadastroConcorrenteDoMesmoEmail_RecusaComoCredencialInvalida(t *testing.T) {
	clock := defaultClock()
	repo := racingRepository{newFakeRepository(clock)}
	svc := setupAuthService(repo, true, clock)

	_, err := svc.LoginLocal(context.Background(), "corrida@markupp.dev", "senha")

	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func servicoComEmail(repo auth.Repository, emailEnabled bool) *auth.Service {
	clock := defaultClock()
	cfg := auth.ServiceConfig{
		DefaultTenantID: "default", RegistrationEnabled: true, LocalEnabled: true,
		RefreshExpirationPeriod: 7 * 24 * time.Hour, EmailEnabled: emailEnabled,
	}
	return auth.NewService(repo, auth.NewTokenCodec("segredo-para-testes-de-servico", 15*time.Minute, clock), &fakeHasher{}, cfg, clock)
}

func TestService_AutoRegistroComSMTP_PedeAVerificacaoDoEmail(t *testing.T) {
	repo := newFakeRepository(defaultClock())

	_, err := servicoComEmail(repo, true).LoginLocal(context.Background(), "nova@markupp.dev", "senhaForte123")

	require.NoError(t, err)
	assert.True(t, repo.lastAccount.SendVerification)
}

func TestService_AutoRegistroSemSMTP_NaoPedeVerificacao(t *testing.T) {
	repo := newFakeRepository(defaultClock())

	_, err := servicoComEmail(repo, false).LoginLocal(context.Background(), "nova@markupp.dev", "senhaForte123")

	require.NoError(t, err)
	assert.False(t, repo.lastAccount.SendVerification)
}
