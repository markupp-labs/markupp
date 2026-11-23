package auth

import (
	"crypto/sha256"
	"crypto/subtle"
)

// SameSecret compara os resumos SHA-256 dos dois segredos em tempo constante. Os resumos
// têm sempre o mesmo tamanho, então nem o tamanho do segredo nem um segredo configurado
// vazio mudam o tempo da comparação.
func SameSecret(received, configured string) bool {
	receivedSum := sha256.Sum256([]byte(received))
	configuredSum := sha256.Sum256([]byte(configured))
	return subtle.ConstantTimeCompare(receivedSum[:], configuredSum[:]) == 1
}
