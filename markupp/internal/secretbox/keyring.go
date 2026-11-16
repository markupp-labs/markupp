package secretbox

import (
	"fmt"
)

// Keyring sela com a chave ativa e abre também com as antigas, para a instância trocar a
// chave de cifra sem perder o que já estava selado (ADR-0057).
type Keyring struct {
	active    *Box
	fallbacks []*Box
}

// NewKeyring monta o chaveiro com a chave active e as antigas fallbacks. A antiga igual
// à ativa, ou vazia, fica de fora.
func NewKeyring(active string, fallbacks ...string) (*Keyring, error) {
	box, err := New(active)
	if err != nil {
		return nil, err
	}
	ring := &Keyring{active: box}
	for _, key := range fallbacks {
		if key == "" || key == active {
			continue
		}
		old, err := New(key)
		if err != nil {
			return nil, err
		}
		ring.fallbacks = append(ring.fallbacks, old)
	}
	return ring, nil
}

// Seal cifra plain com a chave ativa.
func (k *Keyring) Seal(plain string) (string, error) { return k.active.Seal(plain) }

// Open decifra com a chave ativa e, se ela não abre, com as antigas. stale diz que abriu
// com uma antiga, e o texto precisa ser selado de novo com a ativa.
func (k *Keyring) Open(sealed string) (plain string, stale bool, err error) {
	plain, activeErr := k.active.Open(sealed)
	if activeErr == nil {
		return plain, false, nil
	}
	for _, old := range k.fallbacks {
		if plain, err := old.Open(sealed); err == nil {
			return plain, true, nil
		}
	}
	return "", false, fmt.Errorf("no key in the keyring opens the sealed secret, expected the active or a previous key: %w", activeErr)
}
