// Package secretbox cifra os segredos que o banco guarda, como a chave do provedor de
// embedding, com AES-256-GCM e nonce aleatório (ADR-0057).
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// Box sela e abre textos com a chave de cifra da instância.
type Box struct {
	aead cipher.AEAD
}

// New monta a caixa sobre key. A chave AES de 256 bits é o SHA-256 de key, que já é
// um segredo aleatório da instância.
func New(key string) (*Box, error) {
	if key == "" {
		return nil, errors.New("secretbox key is empty, expected the instance key-encryption secret")
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("start AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("start GCM mode: %w", err)
	}
	return &Box{aead: aead}, nil
}

// Seal cifra plain e devolve o nonce seguido do texto cifrado, em base64.
func (b *Box) Seal(plain string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("read random nonce: %w", err)
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Open decifra o que Seal devolveu, e recusa o texto alterado ou de outra chave.
func (b *Box) Open(sealed string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", fmt.Errorf("sealed secret is not base64, expected the output of Seal: %w", err)
	}
	size := b.aead.NonceSize() + b.aead.Overhead()
	if len(raw) < size {
		return "", fmt.Errorf("sealed secret has %d bytes, expected at least %d", len(raw), size)
	}
	plain, err := b.aead.Open(nil, raw[:b.aead.NonceSize()], raw[b.aead.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("open sealed secret, expected one sealed with the same key: %w", err)
	}
	return string(plain), nil
}
