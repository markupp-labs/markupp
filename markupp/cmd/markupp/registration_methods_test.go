package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

func provedoresVigentes(settings ...loginprovider.Settings) *loginprovider.Registry {
	return loginprovider.NewRegistry(settings, http.DefaultClient)
}

func TestRegistrationMethods_SenhaEProvedores_NaOrdemComATravaDeCadaUm(t *testing.T) {
	fechado := false
	cfg := config.Config{
		LocalAuthEnabled: true, LocalRegistration: &fechado,
		LoginProviders: []config.LoginProvider{{ID: "github", Name: "GitHub"}, {ID: "empresa", Name: "Empresa", Registration: &fechado}},
	}
	providers := provedoresVigentes(loginprovider.Settings{ID: "github", Name: "GitHub"},
		loginprovider.Settings{ID: "empresa", Name: "Empresa", Issuer: "https://sso.empresa.exemplo"})

	assert.Equal(t, []instance.RegistrationMethod{
		{ID: "local", Name: "Email and password", HostSetting: &fechado},
		{ID: "github", Name: "GitHub"},
		{ID: "empresa", Name: "Empresa", HostSetting: &fechado},
	}, newRegistrationMethods(cfg, providers).RegistrationMethods())
}

func TestRegistrationMethods_SenhaDesligada_SoOsProvedores(t *testing.T) {
	cfg := config.Config{LocalAuthEnabled: false}

	assert.Equal(t, []instance.RegistrationMethod{{ID: "github", Name: "GitHub"}},
		newRegistrationMethods(cfg, provedoresVigentes(loginprovider.Settings{ID: "github", Name: "GitHub"})).RegistrationMethods())
}

// O provedor que o administrador cria pelo painel vira método de cadastro sem reiniciar (ADR-0060).
func TestRegistrationMethods_ProvedorDoBanco_EntraSemTrava(t *testing.T) {
	cfg := config.Config{LocalAuthEnabled: true}

	methods := newRegistrationMethods(cfg, provedoresVigentes(loginprovider.Settings{ID: "google", Name: "Google", Issuer: "https://accounts.google.com"})).RegistrationMethods()

	assert.Equal(t, instance.RegistrationMethod{ID: "google", Name: "Google"}, methods[1])
}
