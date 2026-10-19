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

// maxStackedMarks limita as marcas combinantes seguidas, que empilhadas (texto zalgo)
// transbordam a linha do nome na tela.
const maxStackedMarks = 2

// SanitizeClientName prepara o client_name declarado pelo cliente, por documento CIMD ou
// por registro dinâmico, para a tela de consentimento: remove controles (Cc) e
// caracteres de formatação (Cf, como a inversão de direção do texto), junta os espaços,
// recusa UTF-8 inválido, mais de maxStackedMarks marcas combinantes seguidas e mais de
// maxClientNameLen caracteres depois do saneamento. As mensagens de erro são ASCII,
// prontas para o error_description.
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
	if err := checkStackedMarks(name); err != nil {
		return "", err
	}
	return name, nil
}

// clientNameRune guarda os espaços para strings.Fields juntar, inclusive os de controle
// como a quebra de linha, e remove os demais controles e os de formatação.
func clientNameRune(r rune) rune {
	if unicode.IsSpace(r) {
		return ' '
	}
	if unicode.In(r, unicode.Cc, unicode.Cf) {
		return -1
	}
	return r
}

// checkStackedMarks recusa o nome com mais de maxStackedMarks marcas combinantes
// seguidas, contadas depois do saneamento.
func checkStackedMarks(name string) error {
	stacked := 0
	for position, r := range []rune(name) {
		if !unicode.Is(unicode.M, r) {
			stacked = 0
			continue
		}
		stacked++
		if stacked > maxStackedMarks {
			return fmt.Errorf("client_name com mais de %d marcas combinantes seguidas no caractere %d; esperado no maximo %d", maxStackedMarks, position+1, maxStackedMarks)
		}
	}
	return nil
}
