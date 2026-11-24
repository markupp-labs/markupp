package auth_test

import "context"

// FakeRegistrationPolicy abre o cadastro só para os métodos em open, ou falha com err.
type FakeRegistrationPolicy struct {
	open map[string]bool
	err  error
}

func (f FakeRegistrationPolicy) RegistrationOpen(_ context.Context, method string) (bool, error) {
	return f.open[method], f.err
}

// registrationPolicy abre senha e GitHub quando enabled, e fecha tudo quando não.
func registrationPolicy(enabled bool) FakeRegistrationPolicy {
	if !enabled {
		return FakeRegistrationPolicy{}
	}
	return FakeRegistrationPolicy{open: map[string]bool{"local": true, "github": true}}
}
