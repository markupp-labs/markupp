package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

// Erros do login por provedor externo (ADR-0054).
var (
	ErrUnknownLoginProvider = errors.New("login provider not configured")
	ErrIdentityNotLinked    = errors.New("no account uses this identity")
	ErrProviderEmailMissing = errors.New("login provider did not share an email")
)

// localProvider é o provedor da identidade de email e senha.
const localProvider = "local"

// LoginProviderLookup são os provedores configurados.
type LoginProviderLookup interface {
	List() []loginprovider.Provider
	Get(id string) (loginprovider.Provider, bool)
}

// LoginProviderSummary é o que o cliente precisa para mostrar o botão do provedor.
type LoginProviderSummary struct {
	ID   string
	Name string
}

// ExternalCallback é a volta do provedor: o código e o que o cliente guardou do Begin.
type ExternalCallback struct {
	Provider     string
	Code         string
	CodeVerifier string
	Nonce        string
}

// ExternalLoginService entra, cadastra e vincula contas por provedor externo, com as
// regras e a sessão do Service. O login nunca cria conta, e o vínculo só nasce de uma
// sessão ativa, nunca pelo email do provedor.
type ExternalLoginService struct {
	base      *Service
	providers LoginProviderLookup
}

// NewExternalLoginService monta o serviço sobre base e os provedores configurados.
func NewExternalLoginService(base *Service, providers LoginProviderLookup) *ExternalLoginService {
	return &ExternalLoginService{base: base, providers: providers}
}

// Providers lista os provedores configurados, na ordem da configuração.
func (s *ExternalLoginService) Providers() []LoginProviderSummary {
	out := []LoginProviderSummary{}
	for _, p := range s.providers.List() {
		out = append(out, LoginProviderSummary{ID: p.ID(), Name: p.Name()})
	}
	return out
}

func (s *ExternalLoginService) provider(id string) (loginprovider.Provider, error) {
	p, ok := s.providers.Get(id)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownLoginProvider, id)
	}
	return p, nil
}

// Begin começa a tentativa no provedor providerID.
func (s *ExternalLoginService) Begin(ctx context.Context, providerID string) (loginprovider.Attempt, error) {
	p, err := s.provider(providerID)
	if err != nil {
		return loginprovider.Attempt{}, err
	}
	return p.Begin(ctx)
}

// finish troca o código do callback pela identidade no provedor.
func (s *ExternalLoginService) finish(ctx context.Context, cb ExternalCallback) (loginprovider.Identity, error) {
	p, err := s.provider(cb.Provider)
	if err != nil {
		return loginprovider.Identity{}, err
	}
	return p.Finish(ctx, cb.Code, cb.CodeVerifier, cb.Nonce)
}

// Login abre a sessão da conta que tem a identidade vinculada.
func (s *ExternalLoginService) Login(ctx context.Context, cb ExternalCallback) (TokenPair, error) {
	ident, err := s.finish(ctx, cb)
	if err != nil {
		return TokenPair{}, err
	}
	linked, err := s.base.repo.GetUserIdentity(ctx, cb.Provider, ident.Subject)
	if errors.Is(err, ErrIdentityNotFound) {
		if auditErr := s.base.auditLoginAttempt(ctx, "", cb.Provider+":"+ident.Subject, "failure"); auditErr != nil {
			return TokenPair{}, fmt.Errorf("audit unlinked login of %s: %w", cb.Provider, auditErr)
		}
		return TokenPair{}, fmt.Errorf("%w: %s:%s", ErrIdentityNotLinked, cb.Provider, ident.Subject)
	}
	if err != nil {
		return TokenPair{}, fmt.Errorf("find identity %s:%s: %w", cb.Provider, ident.Subject, err)
	}
	user, err := s.base.repo.GetUserByID(ctx, linked.UserID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("find user %q: %w", linked.UserID, err)
	}
	return s.base.issueAndPersist(ctx, user, "auth.login")
}

// Register cria a conta com o email do provedor e a identidade sem senha, e abre a
// sessão. Segue o auto-registro, como o cadastro por senha (ADR-0052).
func (s *ExternalLoginService) Register(ctx context.Context, cb ExternalCallback) (TokenPair, error) {
	ident, err := s.finish(ctx, cb)
	if err != nil {
		return TokenPair{}, err
	}
	if ident.Email == "" {
		return TokenPair{}, fmt.Errorf("%w: %s:%s", ErrProviderEmailMissing, cb.Provider, ident.Subject)
	}
	if err := s.ensureUnlinked(ctx, cb.Provider, ident.Subject); err != nil {
		return TokenPair{}, err
	}
	user, err := s.base.repo.CreateAccountTx(ctx, s.accountParams(cb.Provider, ident))
	if err != nil {
		return TokenPair{}, fmt.Errorf("create account for %s:%s: %w", cb.Provider, ident.Subject, err)
	}
	return s.base.issueAndPersist(ctx, user, "auth.login")
}

func (s *ExternalLoginService) accountParams(provider string, ident loginprovider.Identity) CreateAccountParams {
	p := s.base.buildAccountParams(normalizeEmail(ident.Email), "")
	p.Identity.Provider, p.Identity.ProviderUserID = provider, ident.Subject
	p.EmailVerified = ident.EmailVerified
	p.SendVerification = s.base.cfg.EmailEnabled && !ident.EmailVerified
	return p
}

// ensureUnlinked recusa a identidade que já pertence a uma conta.
func (s *ExternalLoginService) ensureUnlinked(ctx context.Context, provider, subject string) error {
	_, err := s.base.repo.GetUserIdentity(ctx, provider, subject)
	if err == nil {
		return fmt.Errorf("%w: %s:%s", ErrIdentityAlreadyLinked, provider, subject)
	}
	if !errors.Is(err, ErrIdentityNotFound) {
		return fmt.Errorf("find identity %s:%s: %w", provider, subject, err)
	}
	return nil
}

// Link vincula a identidade do callback à conta userID, que está logada.
func (s *ExternalLoginService) Link(ctx context.Context, userID string, cb ExternalCallback) (UserIdentity, error) {
	ident, err := s.finish(ctx, cb)
	if err != nil {
		return UserIdentity{}, err
	}
	linked := UserIdentity{ID: s.base.newID(), UserID: userID, Provider: cb.Provider, ProviderUserID: ident.Subject, CreatedAt: s.base.clock()}
	if err := s.base.repo.LinkIdentityTx(ctx, linked, s.identityEvent(userID, "auth.identity_link", cb.Provider)); err != nil {
		return UserIdentity{}, err
	}
	return linked, nil
}

// ListIdentities devolve as identidades externas da conta; a senha fica de fora.
func (s *ExternalLoginService) ListIdentities(ctx context.Context, userID string) ([]UserIdentity, error) {
	all, err := s.base.repo.ListUserIdentities(ctx, userID)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(i UserIdentity) bool { return i.Provider == localProvider }), nil
}

// Unlink desvincula a identidade externa da conta, recusando quando nenhuma das que
// restam serve para entrar nesta instalação.
func (s *ExternalLoginService) Unlink(ctx context.Context, userID, identityID string) error {
	all, err := s.base.repo.ListUserIdentities(ctx, userID)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(all, func(ident UserIdentity) bool { return ident.ID == identityID && ident.Provider != localProvider })
	if i < 0 {
		return fmt.Errorf("%w: identity %q of user %q", ErrIdentityNotFound, identityID, userID)
	}
	if !slices.ContainsFunc(slices.Delete(slices.Clone(all), i, i+1), s.usable) {
		return fmt.Errorf("%w: identity %q of user %q", ErrLastIdentity, identityID, userID)
	}
	return s.base.repo.UnlinkIdentityTx(ctx, userID, identityID, s.identityEvent(userID, "auth.identity_unlink", all[i].Provider))
}

// usable diz se a identidade serve para entrar hoje: senha ligada ou provedor configurado.
func (s *ExternalLoginService) usable(ident UserIdentity) bool {
	if ident.Provider == localProvider {
		return s.base.cfg.LocalEnabled
	}
	_, ok := s.providers.Get(ident.Provider)
	return ok
}

func (s *ExternalLoginService) identityEvent(userID, action, provider string) AuditEventRecord {
	return AuditEventRecord{ID: s.base.newID(), ActorID: userID, Action: action, Target: provider, Result: "success", OccurredAt: s.base.clock()}
}
