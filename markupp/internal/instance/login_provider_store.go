package instance

import (
	"context"
	"errors"
	"time"
)

// Erros do repositório dos provedores de login da instância.
var (
	ErrLoginProviderExists   = errors.New("login provider id already exists")
	ErrLoginProviderNotFound = errors.New("login provider not found")
)

// LoginProviderRecord é um provedor de login guardado pelo administrador (ADR-0060), com o
// segredo do cliente selado. LinkedAccounts conta as contas vinculadas a ele.
type LoginProviderRecord struct {
	ID             string
	Name           string
	Issuer         string
	ClientID       string
	SealedSecret   string
	Enabled        bool
	LinkedAccounts int
	UpdatedAt      time.Time
}

// LoginProviderStore persiste os provedores de login da instância.
type LoginProviderStore interface {
	ListLoginProviders(ctx context.Context) ([]LoginProviderRecord, error)
	CreateLoginProvider(ctx context.Context, record LoginProviderRecord) error
	UpdateLoginProvider(ctx context.Context, record LoginProviderRecord) error
	// DeleteLoginProvider apaga o provedor com os vínculos das contas e a escolha do
	// cadastro dele, e diz quantos vínculos saíram.
	DeleteLoginProvider(ctx context.Context, id string) (int, error)
	// ResealLoginProviderSecret troca o selo old por fresh, sem mudar a revisão.
	ResealLoginProviderSecret(ctx context.Context, id, old, fresh string) error
}
