// Package vaults reúne a gestão de cofres: criar, renomear, apagar e escolher quem é
// membro, sempre no tenant do ator e só nos cofres de que ele é membro (ADR-0024).
package vaults

import (
	"errors"
	"time"
)

// Erros da gestão de cofres.
var (
	ErrVaultNotFound      = errors.New("cofre inexistente ou fora do alcance da conta")
	ErrMemberNotFound     = errors.New("conta nao e membro do cofre")
	ErrNotTenantMember    = errors.New("conta nao pertence ao tenant")
	ErrAlreadyVaultMember = errors.New("conta ja e membro do cofre")
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
