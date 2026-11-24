package instance

import (
	"context"
	"errors"
	"fmt"
)

// Erros da escolha do cadastro por método.
var (
	ErrRegistrationLocked        = errors.New("registration for this method is set by the host configuration")
	ErrUnknownRegistrationMethod = errors.New("unknown registration method")
)

// RegistrationMethod é um método de cadastro configurado: email e senha ou um provedor
// externo. HostSetting, quando definido, é o valor que quem hospeda fixou.
type RegistrationMethod struct {
	ID          string
	Name        string
	HostSetting *bool
}

// RegistrationMethodSource dá os métodos de cadastro vigentes, que mudam com os provedores
// de login do administrador (ADR-0060).
type RegistrationMethodSource interface {
	RegistrationMethods() []RegistrationMethod
}

// RegistrationMethodList é uma lista fixa de métodos.
type RegistrationMethodList []RegistrationMethod

// RegistrationMethods devolve a própria lista.
func (l RegistrationMethodList) RegistrationMethods() []RegistrationMethod { return l }

// RegistrationMethodView é o que a API mostra de cada método.
type RegistrationMethodView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Open   bool   `json:"open"`
	Locked bool   `json:"locked"`
}

// RegistrationService diz quais métodos aceitam cadastro e grava a escolha do
// administrador da instância (ADR-0059).
type RegistrationService struct {
	store   RegistrationStore
	enabled bool
	methods RegistrationMethodSource
}

// NewRegistrationService monta o serviço sobre o banco, o interruptor geral e os métodos
// vigentes, na ordem em que aparecem.
func NewRegistrationService(store RegistrationStore, enabled bool, methods RegistrationMethodSource) *RegistrationService {
	return &RegistrationService{store: store, enabled: enabled, methods: methods}
}

// RegistrationEnabled diz se o interruptor geral de quem hospeda deixa cadastrar.
func (s *RegistrationService) RegistrationEnabled() bool { return s.enabled }

// Methods lista cada método com o valor que vale e se ele está travado.
func (s *RegistrationService) Methods(ctx context.Context) ([]RegistrationMethodView, error) {
	choices, err := s.store.LoadRegistrationChoices(ctx)
	if err != nil {
		return nil, err
	}
	methods := s.methods.RegistrationMethods()
	views := make([]RegistrationMethodView, 0, len(methods))
	for _, method := range methods {
		views = append(views, s.view(method, choices))
	}
	return views, nil
}

// SetOpen grava se o método id aceita cadastro, se nada de quem hospeda o trava.
func (s *RegistrationService) SetOpen(ctx context.Context, id string, open bool) (RegistrationMethodView, error) {
	method, ok := s.method(id)
	if !ok {
		return RegistrationMethodView{}, fmt.Errorf("%w: %q, expected one of the configured methods", ErrUnknownRegistrationMethod, id)
	}
	if s.view(method, nil).Locked {
		return RegistrationMethodView{}, fmt.Errorf("%w: %q", ErrRegistrationLocked, id)
	}
	if err := s.store.SaveRegistrationChoice(ctx, id, open); err != nil {
		return RegistrationMethodView{}, err
	}
	return s.view(method, map[string]bool{id: open}), nil
}

// RegistrationOpen diz se o método id aceita cadastro agora; método desconhecido não aceita.
// Sem conta na instalação, todo método configurado aceita a primeira (ADR-0052).
func (s *RegistrationService) RegistrationOpen(ctx context.Context, id string) (bool, error) {
	method, ok := s.method(id)
	if !ok {
		return false, nil
	}
	hasAccounts, err := s.store.HasAccounts(ctx)
	if err != nil {
		return false, err
	}
	if !hasAccounts || !s.enabled {
		return !hasAccounts, nil
	}
	choices, err := s.store.LoadRegistrationChoices(ctx)
	if err != nil {
		return false, err
	}
	return s.view(method, choices).Open, nil
}

func (s *RegistrationService) method(id string) (RegistrationMethod, bool) {
	for _, method := range s.methods.RegistrationMethods() {
		if method.ID == id {
			return method, true
		}
	}
	return RegistrationMethod{}, false
}

// view resolve o valor do método: o interruptor geral desligado fecha e trava tudo, a
// variável de quem hospeda vence a escolha, e sem escolha o método está aberto.
func (s *RegistrationService) view(method RegistrationMethod, choices map[string]bool) RegistrationMethodView {
	view := RegistrationMethodView{ID: method.ID, Name: method.Name, Open: true}
	switch {
	case !s.enabled:
		view.Open, view.Locked = false, true
	case method.HostSetting != nil:
		view.Open, view.Locked = *method.HostSetting, true
	default:
		if open, ok := choices[method.ID]; ok {
			view.Open = open
		}
	}
	return view
}
