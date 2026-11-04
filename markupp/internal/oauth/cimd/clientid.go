package cimd

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrInvalidClientID indica um client_id que não serve como URL de metadados.
var ErrInvalidClientID = errors.New("invalid client_id")

const clientIDShape = "an https URL with host and path, without query, fragment, credentials or . and .. segments"

// ParseClientID valida o client_id antes de qualquer busca e devolve a URL interpretada.
// A porta fica para o Fetcher, que só aceita a 443; a query não passa, nem vazia, embora
// o draft só a desaconselhe.
func ParseClientID(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, invalidClientID(raw, "is not a URL")
	}
	if reason := clientIDProblem(raw, parsed); reason != "" {
		return nil, invalidClientID(raw, reason)
	}
	return parsed, nil
}

func clientIDProblem(raw string, parsed *url.URL) string {
	switch {
	case parsed.Scheme != "https":
		return "uses a scheme other than https"
	case parsed.Hostname() == "":
		return "has no host"
	case parsed.User != nil:
		return "contains credentials"
	case strings.Contains(raw, "?"):
		return "contains a query"
	case strings.Contains(raw, "#"):
		return "contains a fragment"
	case parsed.Path == "" || parsed.Path == "/":
		return "has no path"
	case hasDotSegment(parsed.Path):
		return "contains a . or .. segment"
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
	return fmt.Errorf("%w: %q %s; expected %s", ErrInvalidClientID, raw, reason, clientIDShape)
}
