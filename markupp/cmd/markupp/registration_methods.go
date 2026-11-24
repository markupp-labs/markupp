package main

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

// newRegistrationService monta o cadastro por método (ADR-0059) sobre o banco, o
// interruptor geral e os métodos configurados.
func newRegistrationService(cfg config.Config, pool *pgxpool.Pool) *instance.RegistrationService {
	return instance.NewRegistrationService(storage.NewInstanceRegistration(pool), cfg.AuthRegistrationEnabled, instance.RegistrationMethodList(registrationMethodsFrom(cfg)))
}

// registrationMethodsFrom lista email e senha, quando a credencial local vale, e os
// provedores, cada um com a trava de quem hospeda.
func registrationMethodsFrom(cfg config.Config) []instance.RegistrationMethod {
	methods := make([]instance.RegistrationMethod, 0, len(cfg.LoginProviders)+1)
	if cfg.LocalAuthEnabled {
		methods = append(methods, instance.RegistrationMethod{ID: "local", Name: "Email and password", HostSetting: cfg.LocalRegistration})
	}
	for _, p := range cfg.LoginProviders {
		methods = append(methods, instance.RegistrationMethod{ID: p.ID, Name: p.Name, HostSetting: p.Registration})
	}
	return methods
}
