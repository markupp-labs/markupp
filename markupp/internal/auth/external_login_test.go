package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

// fakeLoginProvider faz o papel de um provedor externo que devolve a identidade ou o
// erro que o teste escolhe, sem rede.
type fakeLoginProvider struct {
	id       string
	identity loginprovider.Identity
	err      error
	finished []string
}

func (p *fakeLoginProvider) ID() string   { return p.id }
func (p *fakeLoginProvider) Name() string { return "Nome de " + p.id }

func (p *fakeLoginProvider) Begin(_ context.Context) (loginprovider.Attempt, error) {
	return loginprovider.Attempt{AuthorizationURL: "https://" + p.id + ".exemplo.org/authorize", State: "s", CodeVerifier: "v", Nonce: "n"}, nil
}

func (p *fakeLoginProvider) Finish(_ context.Context, code, verifier, nonce string) (loginprovider.Identity, error) {
	p.finished = append(p.finished, code+"|"+verifier+"|"+nonce)
	return p.identity, p.err
}

// fakeLoginProviders é a lista de provedores configurados.
type fakeLoginProviders []loginprovider.Provider

func (f fakeLoginProviders) List() []loginprovider.Provider { return f }

func (f fakeLoginProviders) Get(id string) (loginprovider.Provider, bool) {
	for _, p := range f {
		if p.ID() == id {
			return p, true
		}
	}
	return nil, false
}

type externalFixture struct {
	repo   *fakeRepository
	github *fakeLoginProvider
	svc    *auth.ExternalLoginService
}

func newExternalFixture(t *testing.T, cfg auth.ServiceConfig) externalFixture {
	t.Helper()
	clock := defaultClock()
	repo := newFakeRepository(clock)
	github := &fakeLoginProvider{id: "github", identity: loginprovider.Identity{Subject: "42", Email: "Ana@Exemplo.org", EmailVerified: true}}
	cfg.DefaultTenantID, cfg.RefreshExpirationPeriod = "default", time.Hour
	base := auth.NewService(repo, auth.NewTokenCodec("segredo-para-testes-de-servico", time.Minute, clock), &fakeHasher{}, cfg, clock)
	return externalFixture{repo: repo, github: github, svc: auth.NewExternalLoginService(base, fakeLoginProviders{github})}
}

func callback() auth.ExternalCallback {
	return auth.ExternalCallback{Provider: "github", Code: "codigo", CodeVerifier: "verifier", Nonce: "nonce"}
}

// withUser grava uma conta com as identidades dadas, todas dela.
func (f externalFixture) withUser(id string, identities ...auth.UserIdentity) {
	f.repo.users[id] = auth.User{ID: id, Email: id + "@exemplo.org"}
	for _, ident := range identities {
		ident.UserID = id
		f.repo.identities[ident.Provider+":"+ident.ProviderUserID] = ident
	}
}

func localIdentity(id string) auth.UserIdentity {
	return auth.UserIdentity{ID: id, Provider: "local", ProviderUserID: id + "@exemplo.org", PasswordHash: "hash"}
}

func githubIdentity(id, sub string) auth.UserIdentity {
	return auth.UserIdentity{ID: id, Provider: "github", ProviderUserID: sub, CreatedAt: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)}
}

func TestExternalLogin_Providers_ListaIdENome(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{})

	assert.Equal(t, []auth.LoginProviderSummary{{ID: "github", Name: "Nome de github"}}, f.svc.Providers())
}

func TestExternalLogin_Begin_DevolveATentativaDoProvedor(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{})

	attempt, err := f.svc.Begin(context.Background(), "github")

	require.NoError(t, err)
	assert.Equal(t, "https://github.exemplo.org/authorize", attempt.AuthorizationURL)
}

func TestExternalLogin_ProvedorDesconhecido_Recusa(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{})
	cb := callback()
	cb.Provider = "gitlab"

	_, beginErr := f.svc.Begin(context.Background(), "gitlab")
	_, loginErr := f.svc.Login(context.Background(), cb)

	require.ErrorIs(t, beginErr, auth.ErrUnknownLoginProvider)
	require.ErrorIs(t, loginErr, auth.ErrUnknownLoginProvider)
}

func TestExternalLogin_Login_IdentidadeVinculada_AbreASessao(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{})
	f.withUser("user-1", githubIdentity("ident-gh", "42"))

	pair, err := f.svc.Login(context.Background(), callback())

	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.Equal(t, []string{"codigo|verifier|nonce"}, f.github.finished)
	last := f.repo.auditEvents[len(f.repo.auditEvents)-1]
	assert.Equal(t, "auth.login", last.Action)
	assert.Equal(t, "user-1", last.ActorID)
}

func TestExternalLogin_Login_SemVinculo_RecusaENaoCriaConta(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{RegistrationEnabled: true})

	_, err := f.svc.Login(context.Background(), callback())

	require.ErrorIs(t, err, auth.ErrIdentityNotLinked)
	assert.Empty(t, f.repo.users, "login nunca cria conta (ADR-0052)")
	assert.Equal(t, "failure", f.repo.auditEvents[len(f.repo.auditEvents)-1].Result)
}

func TestExternalLogin_Login_ProvedorRecusa_PropagaARecusa(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{})
	f.github.err = loginprovider.ErrLoginRejected

	_, err := f.svc.Login(context.Background(), callback())

	require.ErrorIs(t, err, loginprovider.ErrLoginRejected)
}

func TestExternalLogin_Register_CriaContaSemSenhaComOEmailDoProvedor(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{EmailEnabled: true})

	pair, err := f.svc.Register(context.Background(), callback())

	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	p := f.repo.lastAccount
	assert.Equal(t, "ana@exemplo.org", p.User.Email)
	assert.Equal(t, "github", p.Identity.Provider)
	assert.Equal(t, "42", p.Identity.ProviderUserID)
	assert.Empty(t, p.Identity.PasswordHash)
	assert.True(t, p.EmailVerified)
	assert.False(t, p.SendVerification, "o provedor ja provou o email")
}

func TestExternalLogin_Register_EmailNaoVerificado_PedeVerificacao(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{EmailEnabled: true})
	f.github.identity.EmailVerified = false

	_, err := f.svc.Register(context.Background(), callback())

	require.NoError(t, err)
	assert.False(t, f.repo.lastAccount.EmailVerified)
	assert.True(t, f.repo.lastAccount.SendVerification)
}

func TestExternalLogin_Register_AutoRegistroDesligado_RecusaDepoisDaPrimeiraConta(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{RegistrationEnabled: false})
	f.withUser("user-1", localIdentity("ident-local"))

	_, err := f.svc.Register(context.Background(), callback())

	require.ErrorIs(t, err, auth.ErrRegistrationDisabled)
}

func TestExternalLogin_Register_SemEmail_Recusa(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{})
	f.github.identity.Email = ""

	_, err := f.svc.Register(context.Background(), callback())

	require.ErrorIs(t, err, auth.ErrProviderEmailMissing)
}

func TestExternalLogin_Register_IdentidadeJaVinculada_Recusa(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{RegistrationEnabled: true})
	f.withUser("user-1", githubIdentity("ident-gh", "42"))

	_, err := f.svc.Register(context.Background(), callback())

	require.ErrorIs(t, err, auth.ErrIdentityAlreadyLinked)
}

func TestExternalLogin_Link_VinculaAContaLogada(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{})
	f.withUser("user-1", localIdentity("ident-local"))

	linked, err := f.svc.Link(context.Background(), "user-1", callback())

	require.NoError(t, err)
	assert.Equal(t, "github", linked.Provider)
	got, err := f.repo.GetUserIdentity(context.Background(), "github", "42")
	require.NoError(t, err)
	assert.Equal(t, "user-1", got.UserID)
	assert.Equal(t, "auth.identity_link", f.repo.auditEvents[len(f.repo.auditEvents)-1].Action)
}

func TestExternalLogin_Link_IdentidadeDeOutraConta_Recusa(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{})
	f.withUser("user-1", localIdentity("ident-local"))
	f.withUser("user-2", githubIdentity("ident-gh", "42"))

	_, err := f.svc.Link(context.Background(), "user-1", callback())

	require.ErrorIs(t, err, auth.ErrIdentityAlreadyLinked)
}

func TestExternalLogin_ListIdentities_DeixaDeForaACredencialLocal(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{})
	f.withUser("user-1", localIdentity("ident-local"), githubIdentity("ident-gh", "42"))

	identities, err := f.svc.ListIdentities(context.Background(), "user-1")

	require.NoError(t, err)
	require.Len(t, identities, 1)
	assert.Equal(t, "ident-gh", identities[0].ID)
}

func TestExternalLogin_Unlink_ComSenhaRestando_Desvincula(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{LocalEnabled: true})
	f.withUser("user-1", localIdentity("ident-local"), githubIdentity("ident-gh", "42"))

	err := f.svc.Unlink(context.Background(), "user-1", "ident-gh")

	require.NoError(t, err)
	_, err = f.repo.GetUserIdentity(context.Background(), "github", "42")
	require.ErrorIs(t, err, auth.ErrIdentityNotFound)
	assert.Equal(t, "auth.identity_unlink", f.repo.auditEvents[len(f.repo.auditEvents)-1].Action)
}

func TestExternalLogin_Unlink_SoSenhaDesligadaRestando_Recusa(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{LocalEnabled: false})
	f.withUser("user-1", localIdentity("ident-local"), githubIdentity("ident-gh", "42"))

	err := f.svc.Unlink(context.Background(), "user-1", "ident-gh")

	require.ErrorIs(t, err, auth.ErrLastIdentity)
}

func TestExternalLogin_Unlink_SoProvedorQueSaiuDaConfiguracaoRestando_Recusa(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{})
	gitlab := auth.UserIdentity{ID: "ident-gl", Provider: "gitlab", ProviderUserID: "7"}
	f.withUser("user-1", gitlab, githubIdentity("ident-gh", "42"))

	err := f.svc.Unlink(context.Background(), "user-1", "ident-gh")

	require.ErrorIs(t, err, auth.ErrLastIdentity)
}

func TestExternalLogin_Unlink_CredencialLocal_NaoAcha(t *testing.T) {
	f := newExternalFixture(t, auth.ServiceConfig{LocalEnabled: true})
	f.withUser("user-1", localIdentity("ident-local"), githubIdentity("ident-gh", "42"))

	err := f.svc.Unlink(context.Background(), "user-1", "ident-local")

	require.ErrorIs(t, err, auth.ErrIdentityNotFound, "a senha nao sai por esta rota")
}
