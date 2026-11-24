package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
)

func TestRegistrationMethodsFrom_SenhaEProvedores_NaOrdemComATravaDeCadaUm(t *testing.T) {
	fechado := false
	cfg := config.Config{
		LocalAuthEnabled: true, LocalRegistration: &fechado,
		LoginProviders: []config.LoginProvider{{ID: "github", Name: "GitHub"}, {ID: "empresa", Name: "Empresa", Registration: &fechado}},
	}

	assert.Equal(t, []instance.RegistrationMethod{
		{ID: "local", Name: "Email and password", HostSetting: &fechado},
		{ID: "github", Name: "GitHub"},
		{ID: "empresa", Name: "Empresa", HostSetting: &fechado},
	}, registrationMethodsFrom(cfg))
}

func TestRegistrationMethodsFrom_SenhaDesligada_SoOsProvedores(t *testing.T) {
	cfg := config.Config{LocalAuthEnabled: false, LoginProviders: []config.LoginProvider{{ID: "github", Name: "GitHub"}}}

	assert.Equal(t, []instance.RegistrationMethod{{ID: "github", Name: "GitHub"}}, registrationMethodsFrom(cfg))
}
