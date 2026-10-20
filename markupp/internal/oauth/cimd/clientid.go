package cimd

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrInvalidClientID indica um client_id que não serve como URL de metadados.
var ErrInvalidClientID = errors.New("client_id invalido")

const clientIDShape = "URL https com host e caminho, sem query, sem fragmento, sem credenciais e sem segmentos . ou .."

// ParseClientID valida o client_id antes de qualquer busca e devolve a URL interpretada.
// A porta fica para o Fetcher, que só aceita a 443; a query não passa, nem vazia, embora
// o draft só a desaconselhe.
func ParseClientID(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, invalidClientID(raw, "nao e uma URL")
	}
	if reason := clientIDProblem(raw, parsed); reason != "" {
		return nil, invalidClientID(raw, reason)
	}
	return parsed, nil
}

func clientIDProblem(raw string, parsed *url.URL) string {
	switch {
	case parsed.Scheme != "https":
		return "esquema diferente de https"
	case parsed.Hostname() == "":
		return "sem host"
	case parsed.User != nil:
		return "contem credenciais"
	case strings.Contains(raw, "?"):
		return "contem query"
	case strings.Contains(raw, "#"):
		return "contem fragmento"
	case parsed.Path == "" || parsed.Path == "/":
		return "sem caminho"
	case hasDotSegment(parsed.Path):
		return "contem segmento . ou .."
	}
	return ""
}

// hasDotSegment olha o caminho já decodificado, então %2e%2e também é pego.
func hasDotSegment(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return true
		}
	}
	return false
}

func invalidClientID(raw, reason string) error {
	return fmt.Errorf("%w: %q %s; esperado %s", ErrInvalidClientID, raw, reason, clientIDShape)
}
