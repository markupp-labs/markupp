package cimd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidDocument indica um documento de metadados que não atende ao draft.
var ErrInvalidDocument = errors.New("documento de metadados invalido")

// ClientMetadata é o que o servidor aproveita do documento, já validado.
type ClientMetadata struct {
	ClientID     string
	Name         string
	RedirectURIs []string
	TTL          time.Duration
}

type metadataDocument struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod *string  `json:"token_endpoint_auth_method"`
}

func parseDocument(body []byte, clientID string) (ClientMetadata, error) {
	var doc metadataDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return ClientMetadata{}, fmt.Errorf("%w: JSON ilegivel em %s: %w; esperado objeto JSON", ErrInvalidDocument, clientID, err)
	}
	if err := doc.validate(clientID); err != nil {
		return ClientMetadata{}, err
	}
	return ClientMetadata{ClientID: doc.ClientID, Name: doc.ClientName, RedirectURIs: doc.RedirectURIs}, nil
}

func (d metadataDocument) validate(clientID string) error {
	if d.ClientID != clientID {
		return fmt.Errorf("%w: client_id %q; esperado igual a URL buscada %q", ErrInvalidDocument, d.ClientID, clientID)
	}
	if strings.TrimSpace(d.ClientName) == "" {
		return fmt.Errorf("%w: client_name %q; esperado texto nao vazio", ErrInvalidDocument, d.ClientName)
	}
	if d.TokenEndpointAuthMethod != nil && *d.TokenEndpointAuthMethod != "none" {
		return fmt.Errorf("%w: token_endpoint_auth_method %q; esperado ausente ou \"none\"",
			ErrInvalidDocument, *d.TokenEndpointAuthMethod)
	}
	return validateRedirectURIs(d.RedirectURIs)
}

func validateRedirectURIs(uris []string) error {
	if len(uris) == 0 {
		return fmt.Errorf("%w: redirect_uris vazio; esperado lista com ao menos uma URL absoluta", ErrInvalidDocument)
	}
	for _, raw := range uris {
		if reason := RedirectURIProblem(raw); reason != "" {
			return fmt.Errorf("%w: redirect_uri %q %s; esperado %s", ErrInvalidDocument, raw, reason, RedirectURIShape)
		}
	}
	return nil
}
