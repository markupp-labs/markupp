package instance

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

// Erros da escolha dos provedores de login.
var (
	ErrLoginProvidersLocked      = errors.New("login providers are set by the host configuration")
	ErrInvalidLoginProvider      = errors.New("invalid login provider")
	ErrLoginProviderIssuerLocked = errors.New("issuer of a login provider with linked accounts cannot change")
)

// hostRevision é a revisão da lista de quem hospeda, que não muda sem reiniciar.
const hostRevision = "host"

// LoginProviderInput é o provedor pedido pelo administrador. ClientSecret nil mantém o
// guardado; na criação ele é obrigatório.
type LoginProviderInput struct {
	ID           string
	Name         string
	Issuer       string
	ClientID     string
	ClientSecret *string
	Enabled      bool
}

// LoginProviderView é o que a API mostra de um provedor: nunca o segredo.
type LoginProviderView struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Issuer         string `json:"issuer"`
	ClientID       string `json:"client_id"`
	SecretSet      bool   `json:"client_secret_set"`
	Enabled        bool   `json:"enabled"`
	Locked         bool   `json:"locked"`
	CallbackURL    string `json:"callback_url"`
	LinkedAccounts int    `json:"linked_accounts"`
}

// LoginProvidersView é a lista, travada quando vem de quem hospeda.
type LoginProvidersView struct {
	Locked    bool                `json:"locked"`
	Providers []LoginProviderView `json:"providers"`
}

// LoginProviderService guarda os provedores de login do administrador da instância e dá
// ao login os vigentes (ADR-0060). Com provedores de quem hospeda, vale só a lista dele.
type LoginProviderService struct {
	store    LoginProviderStore
	sealer   Sealer
	host     []loginprovider.Settings
	callback func(id string) string
}

// NewLoginProviderService monta o serviço sobre o banco, a cifra do segredo, os provedores
// das variáveis de quem hospeda e o callback do painel de cada id.
func NewLoginProviderService(store LoginProviderStore, sealer Sealer, host []loginprovider.Settings, callback func(id string) string) *LoginProviderService {
	return &LoginProviderService{store: store, sealer: sealer, host: host, callback: callback}
}

func (s *LoginProviderService) locked() bool { return len(s.host) > 0 }

// List mostra os provedores, e sela de novo o segredo que abriu com uma chave antiga.
func (s *LoginProviderService) List(ctx context.Context) (LoginProvidersView, error) {
	if s.locked() {
		return s.hostView(), nil
	}
	records, err := s.store.ListLoginProviders(ctx)
	if err != nil {
		return LoginProvidersView{}, err
	}
	view := LoginProvidersView{Providers: make([]LoginProviderView, 0, len(records))}
	for _, r := range records {
		if err := s.resealStale(ctx, r); err != nil {
			return LoginProvidersView{}, err
		}
		view.Providers = append(view.Providers, s.view(r))
	}
	return view, nil
}

func (s *LoginProviderService) hostView() LoginProvidersView {
	view := LoginProvidersView{Locked: true, Providers: make([]LoginProviderView, 0, len(s.host))}
	for _, h := range s.host {
		view.Providers = append(view.Providers, LoginProviderView{ID: h.ID, Name: h.Name, Issuer: h.Issuer, ClientID: h.ClientID,
			SecretSet: true, Enabled: true, Locked: true, CallbackURL: s.callback(h.ID)})
	}
	return view
}

func (s *LoginProviderService) view(r LoginProviderRecord) LoginProviderView {
	return LoginProviderView{ID: r.ID, Name: r.Name, Issuer: r.Issuer, ClientID: r.ClientID, SecretSet: r.SealedSecret != "",
		Enabled: r.Enabled, CallbackURL: s.callback(r.ID), LinkedAccounts: r.LinkedAccounts}
}

func (s *LoginProviderService) resealStale(ctx context.Context, r LoginProviderRecord) error {
	plain, ok := s.staleSecret(r.SealedSecret)
	if !ok {
		return nil
	}
	fresh, err := s.sealer.Seal(plain)
	if err != nil {
		return fmt.Errorf("reseal the secret of login provider %q: %w", r.ID, err)
	}
	return s.store.ResealLoginProviderSecret(ctx, r.ID, r.SealedSecret, fresh)
}

// staleSecret abre o segredo selado com uma chave de cifra antiga. O ilegível fica como
// está, até o administrador gravar outro.
func (s *LoginProviderService) staleSecret(sealed string) (string, bool) {
	plain, stale, err := s.sealer.Open(sealed)
	return plain, err == nil && stale
}

// Create grava um provedor novo.
func (s *LoginProviderService) Create(ctx context.Context, input LoginProviderInput) (LoginProviderView, error) {
	input, err := s.checked(input)
	if err != nil {
		return LoginProviderView{}, err
	}
	if input.ClientSecret == nil || strings.TrimSpace(*input.ClientSecret) == "" {
		return LoginProviderView{}, fmt.Errorf("%w: client_secret of %q is empty, expected the secret registered at the provider", ErrInvalidLoginProvider, input.ID)
	}
	record, err := s.record(input, "")
	if err != nil {
		return LoginProviderView{}, err
	}
	if err := s.store.CreateLoginProvider(ctx, record); err != nil {
		return LoginProviderView{}, err
	}
	return s.view(record), nil
}

// Update troca os campos do provedor input.ID. O issuer só muda sem conta vinculada.
func (s *LoginProviderService) Update(ctx context.Context, input LoginProviderInput) (LoginProviderView, error) {
	input, err := s.checked(input)
	if err != nil {
		return LoginProviderView{}, err
	}
	stored, err := s.stored(ctx, input.ID)
	if err != nil {
		return LoginProviderView{}, err
	}
	if stored.LinkedAccounts > 0 && stored.Issuer != input.Issuer {
		return LoginProviderView{}, fmt.Errorf("%w: %q has %d linked accounts on issuer %q, expected the same issuer or a new provider",
			ErrLoginProviderIssuerLocked, input.ID, stored.LinkedAccounts, stored.Issuer)
	}
	record, err := s.record(input, stored.SealedSecret)
	if err != nil {
		return LoginProviderView{}, err
	}
	if err := s.store.UpdateLoginProvider(ctx, record); err != nil {
		return LoginProviderView{}, err
	}
	record.LinkedAccounts = stored.LinkedAccounts
	return s.view(record), nil
}

// Delete apaga o provedor id com os vínculos das contas, e diz quantos saíram.
func (s *LoginProviderService) Delete(ctx context.Context, id string) (int, error) {
	if s.locked() {
		return 0, fmt.Errorf("%w: delete %q", ErrLoginProvidersLocked, id)
	}
	return s.store.DeleteLoginProvider(ctx, id)
}

func (s *LoginProviderService) stored(ctx context.Context, id string) (LoginProviderRecord, error) {
	records, err := s.store.ListLoginProviders(ctx)
	if err != nil {
		return LoginProviderRecord{}, err
	}
	for _, r := range records {
		if r.ID == id {
			return r, nil
		}
	}
	return LoginProviderRecord{}, fmt.Errorf("%w: %q", ErrLoginProviderNotFound, id)
}

// checked recusa a mudança com a lista travada e o provedor inválido, e põe o id no
// nome vazio.
func (s *LoginProviderService) checked(input LoginProviderInput) (LoginProviderInput, error) {
	if s.locked() {
		return input, fmt.Errorf("%w: change %q", ErrLoginProvidersLocked, input.ID)
	}
	input.Name, input.Issuer, input.ClientID = strings.TrimSpace(input.Name), strings.TrimSpace(input.Issuer), strings.TrimSpace(input.ClientID)
	if input.Name == "" {
		input.Name = input.ID
	}
	return input, validateLoginProvider(input)
}

func validateLoginProvider(input LoginProviderInput) error {
	switch {
	case !config.ValidLoginProviderID(input.ID):
		return fmt.Errorf("%w: id %q, expected lowercase letters, digits and hyphens, other than \"local\"", ErrInvalidLoginProvider, input.ID)
	case !config.ValidLoginIssuer(input.ID, input.Issuer):
		return fmt.Errorf("%w: issuer %q of %q, expected https://host[/path] (http only on localhost); only the id %q goes without issuer",
			ErrInvalidLoginProvider, input.Issuer, input.ID, config.GitHubLoginProviderID)
	case input.ClientID == "":
		return fmt.Errorf("%w: client_id of %q is empty, expected the id registered at the provider", ErrInvalidLoginProvider, input.ID)
	}
	return nil
}

// record monta o registro, selando o segredo novo ou, sem ele ou em branco, mantendo sealed.
func (s *LoginProviderService) record(input LoginProviderInput, sealed string) (LoginProviderRecord, error) {
	record := LoginProviderRecord{ID: input.ID, Name: input.Name, Issuer: input.Issuer, ClientID: input.ClientID, SealedSecret: sealed, Enabled: input.Enabled}
	if input.ClientSecret == nil || strings.TrimSpace(*input.ClientSecret) == "" {
		return record, nil
	}
	fresh, err := s.sealer.Seal(strings.TrimSpace(*input.ClientSecret))
	if err != nil {
		return LoginProviderRecord{}, fmt.Errorf("seal the secret of login provider %q: %w", input.ID, err)
	}
	record.SealedSecret = fresh
	return record, nil
}

// LoginProviders dá ao login os provedores vigentes e a revisão da lista, que muda quando
// ela muda. O provedor cujo segredo não abre fica de fora até o administrador gravar outro.
func (s *LoginProviderService) LoginProviders(ctx context.Context) (string, []loginprovider.Settings, error) {
	if s.locked() {
		return hostRevision, s.hostSettings(), nil
	}
	records, err := s.store.ListLoginProviders(ctx)
	if err != nil {
		return "", nil, err
	}
	var revision strings.Builder
	settings := []loginprovider.Settings{}
	for _, r := range records {
		fmt.Fprintf(&revision, "%s@%d;", r.ID, r.UpdatedAt.UnixNano())
		if secret, _, err := s.sealer.Open(r.SealedSecret); err == nil && r.Enabled {
			settings = append(settings, loginprovider.Settings{ID: r.ID, Name: r.Name, Issuer: r.Issuer, ClientID: r.ClientID,
				ClientSecret: secret, RedirectURL: s.callback(r.ID)})
		}
	}
	return revision.String(), settings, nil
}

func (s *LoginProviderService) hostSettings() []loginprovider.Settings {
	settings := make([]loginprovider.Settings, 0, len(s.host))
	for _, h := range s.host {
		h.RedirectURL = s.callback(h.ID)
		settings = append(settings, h)
	}
	return settings
}
