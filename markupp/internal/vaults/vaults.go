// Package vaults reúne a gestão de cofres: criar, renomear, apagar e escolher quem é
// membro, sempre no tenant do ator e só nos cofres de que ele é membro (ADR-0024).
package vaults

import (
	"errors"
	"time"
)

// Erros da gestão de cofres.
var (
	ErrVaultNotFound      = errors.New("vault not found or not accessible to this account")
	ErrMemberNotFound     = errors.New("account is not a member of the vault")
	ErrNotTenantMember    = errors.New("account does not belong to the tenant")
	ErrAlreadyVaultMember = errors.New("account is already a member of the vault")
	ErrInvalidName        = errors.New("invalid vault name")
	ErrReadOnly           = errors.New("the viewer role can only read")
)

// Vault é o cofre como a conta o vê.
type Vault struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Member é a conta que pertence ao cofre, com o papel dela no tenant.
type Member struct {
	UserID   string
	Email    string
	Role     string
	JoinedAt time.Time
}
