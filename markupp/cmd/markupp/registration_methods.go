package main

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

// newRegistrationService monta o cadastro por método (ADR-0059) sobre o banco, o
// interruptor geral e os métodos vigentes.
func newRegistrationService(cfg config.Config, pool *pgxpool.Pool, providers auth.LoginProviderLookup) *instance.RegistrationService {
	return instance.NewRegistrationService(storage.NewInstanceRegistration(pool), cfg.AuthRegistrationEnabled, newRegistrationMethods(cfg, providers))
}

// registrationMethods são email e senha, quando a credencial local vale, e os provedores
// de login vigentes, cada um com a trava que quem hospeda fixou nas variáveis.
type registrationMethods struct {
	local     []instance.RegistrationMethod
	hostLocks map[string]*bool
	providers auth.LoginProviderLookup
}

func newRegistrationMethods(cfg config.Config, providers auth.LoginProviderLookup) registrationMethods {
	m := registrationMethods{hostLocks: map[string]*bool{}, providers: providers}
	if cfg.LocalAuthEnabled {
		m.local = []instance.RegistrationMethod{{ID: "local", Name: "Email and password", HostSetting: cfg.LocalRegistration}}
	}
	for _, p := range cfg.LoginProviders {
		m.hostLocks[p.ID] = p.Registration
	}
	return m
}

// RegistrationMethods lista a senha e, depois, os provedores na ordem da lista vigente.
func (m registrationMethods) RegistrationMethods() []instance.RegistrationMethod {
	methods := append([]instance.RegistrationMethod(nil), m.local...)
	for _, p := range m.providers.List() {
		methods = append(methods, instance.RegistrationMethod{ID: p.ID(), Name: p.Name(), HostSetting: m.hostLocks[p.ID()]})
	}
	return methods
}
