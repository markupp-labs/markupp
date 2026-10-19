package cimd

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxClientNameLen limita, em caracteres, o nome que a tela de consentimento mostra.
const maxClientNameLen = 100

// SanitizeClientName prepara o client_name declarado pelo cliente, por documento CIMD ou
// por registro dinâmico, para a tela de consentimento: troca controles por espaço,
// descarta caracteres de formatação (como a inversão de direção do texto), junta os
// espaços e limita o tamanho. As mensagens de erro são ASCII, prontas para o
// error_description.
func SanitizeClientName(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", errors.New("client_name com UTF-8 invalido; esperado texto UTF-8")
	}
	name := strings.Join(strings.Fields(strings.Map(clientNameRune, raw)), " ")
	if name == "" {
		return "", fmt.Errorf("client_name ausente ou vazio; esperado nome com 1 a %d caracteres", maxClientNameLen)
	}
	if count := utf8.RuneCountInString(name); count > maxClientNameLen {
		return "", fmt.Errorf("client_name com %d caracteres; esperado no maximo %d", count, maxClientNameLen)
	}
	return name, nil
}

func clientNameRune(r rune) rune {
	switch {
	case unicode.IsControl(r):
		return ' '
	case unicode.Is(unicode.Cf, r):
		return -1
	}
	return r
}
