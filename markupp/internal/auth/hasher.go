package auth

import "golang.org/x/crypto/bcrypt"

// PasswordHasher abstrai a geração e comparação de hashes de senha.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

// BcryptHasher implementa PasswordHasher utilizando bcrypt.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher cria uma instância de BcryptHasher com custo padrão.
func NewBcryptHasher() *BcryptHasher {
	return &BcryptHasher{cost: bcrypt.DefaultCost}
}

// Hash gera o hash bcrypt para a senha fornecida.
func (b *BcryptHasher) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), b.cost)
	return string(bytes), err
}

// Compare valida a senha plana contra o hash bcrypt.
func (b *BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
