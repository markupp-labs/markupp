package auth

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"
)

// minPasswordLength segue o NIST SP 800-63B-4 para senha que é o único fator: 15
// caracteres, sem exigir tipos de caractere.
const minPasswordLength = 15

// commonPasswordList são as senhas de 15 caracteres ou mais do Pwdb_top-1000000.txt do
// SecLists (commit dbfb830d, licença MIT em common_passwords_LICENSE.txt), em minúsculas
// e sem repetição. As mais curtas a regra do comprimento já recusa.
//
//go:embed common_passwords.txt
var commonPasswordList string

var commonPasswords = sync.OnceValue(func() map[string]struct{} {
	set := make(map[string]struct{})
	for _, p := range strings.Split(strings.TrimSpace(commonPasswordList), "\n") {
		set[p] = struct{}{}
	}
	return set
})

// checkNewPassword valida a senha que alguém escolhe: além do formato de qualquer senha,
// o comprimento mínimo e a lista de senhas comuns. O login não passa por aqui, para a
// conta antiga com senha fraca seguir entrando.
func checkNewPassword(password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	if n := utf8.RuneCountInString(password); n < minPasswordLength {
		return fmt.Errorf("password with %d characters, minimum %d: %w", n, minPasswordLength, ErrPasswordTooShort)
	}
	if _, found := commonPasswords()[strings.ToLower(password)]; found {
		return ErrCommonPassword
	}
	return nil
}
