package config

import (
	"fmt"
	"net/mail"
	"strings"
)

// minSecretKeyBytes é o tamanho mínimo da MARKUPP_SECRET_KEY, como o do segredo do JWT.
const minSecretKeyBytes = 32

// readInstanceAdmins lê MARKUPP_INSTANCE_ADMINS, os emails das contas que administram
// a instância, aparados e em minúsculas. Vazia, ninguém administra pelo painel.
func readInstanceAdmins(lookup LookupFunc, target *[]string) error {
	raw, _ := lookup("MARKUPP_INSTANCE_ADMINS")
	for _, entry := range strings.Split(raw, ",") {
		email := strings.ToLower(strings.TrimSpace(entry))
		if email == "" {
			continue
		}
		if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
			return fmt.Errorf("MARKUPP_INSTANCE_ADMINS entry %q, expected a comma-separated list of emails such as ana@example.org", entry)
		}
		*target = append(*target, email)
	}
	return nil
}

// readSecretKey lê MARKUPP_SECRET_KEY, a chave que cifra os segredos guardados no
// banco. Vazia, a chave vem do próprio banco (ADR-0057).
func readSecretKey(lookup LookupFunc, target *string) error {
	val, _ := lookup("MARKUPP_SECRET_KEY")
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) < minSecretKeyBytes {
		return fmt.Errorf("MARKUPP_SECRET_KEY has %d bytes, expected at least %d bytes of random data", len(trimmed), minSecretKeyBytes)
	}
	*target = trimmed
	return nil
}
