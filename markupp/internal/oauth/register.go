package oauth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth/cimd"
)

// Códigos de erro do registro dinâmico (RFC 7591, seção 3.2.2) e da recusa temporária.
const (
	errInvalidClientMetadata  = "invalid_client_metadata"
	errInvalidRedirectURI     = "invalid_redirect_uri"
	errTemporarilyUnavailable = "temporarily_unavailable"
)

// dcrClientIDPrefix marca o client_id emitido pelo registro dinâmico, que nunca é URL
// https e por isso nunca cai no caminho do CIMD.
const dcrClientIDPrefix = "markupp-dcr-"

// maxClientNameLen limita, em caracteres, o nome que a tela de consentimento mostra.
const maxClientNameLen = 100

// dcrGrantTypes são os grants que o cliente registrado pode usar, na ordem da resposta.
func dcrGrantTypes() []string { return []string{"authorization_code", "refresh_token"} }

// RegisterClient registra um cliente público por DCR (RFC 7591), sem client_secret nem
// registration_access_token. O teto de pendentes atingido vira temporarily_unavailable
// com RetryAfter até a próxima vaga.
func (s *Service) RegisterClient(ctx context.Context, req RegistrationRequest) (RegistrationResponse, error) {
	client, err := checkRegistration(req)
	if err != nil {
		return RegistrationResponse{}, err
	}
	client.ID = dcrClientIDPrefix + s.newToken()
	now := s.clock()
	err = s.repo.RegisterDCRClient(ctx, DCRRegistration{
		Client: client, IssuedAt: now, MaxPending: s.cfg.DCRMaxPending, PendingTTL: s.cfg.DCRPendingTTL,
	})
	if err != nil {
		return RegistrationResponse{}, registrationFailure(err, now)
	}
	return registrationResponse(client, now), nil
}

// checkRegistration confere os metadados do cliente público e devolve o cliente ainda
// sem ID, com o nome saneado.
func checkRegistration(req RegistrationRequest) (Client, error) {
	if err := checkPublicClientMetadata(req); err != nil {
		return Client{}, err
	}
	name, err := cleanClientName(req.ClientName)
	if err != nil {
		return Client{}, err
	}
	if err := checkDCRRedirectURIs(req.RedirectURIs); err != nil {
		return Client{}, err
	}
	return Client{Name: name, RedirectURIs: req.RedirectURIs, Source: ClientSourceDCR}, nil
}

// checkPublicClientMetadata aceita só o cliente público do fluxo authorization code.
func checkPublicClientMetadata(req RegistrationRequest) error {
	switch {
	case req.TokenEndpointAuthMethod != nil && *req.TokenEndpointAuthMethod != "none":
		return invalidClientMetadata("token_endpoint_auth_method " + ClientValue(*req.TokenEndpointAuthMethod) + "; esperado ausente ou none")
	case slices.ContainsFunc(req.GrantTypes, func(g string) bool { return !slices.Contains(dcrGrantTypes(), g) }):
		return invalidClientMetadata("grant_types " + ClientValue(strings.Join(req.GrantTypes, " ")) + "; esperado ausente ou subconjunto de authorization_code e refresh_token")
	case req.ResponseTypes != nil && !slices.Equal(req.ResponseTypes, []string{"code"}):
		return invalidClientMetadata("response_types " + ClientValue(strings.Join(req.ResponseTypes, " ")) + "; esperado ausente ou so code")
	}
	return nil
}

// cleanClientName troca controles por espaço, descarta caracteres de formatação (como a
// inversão de direção do texto), junta os espaços e limita o tamanho.
func cleanClientName(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", invalidClientMetadata("client_name com UTF-8 invalido; esperado texto UTF-8")
	}
	name := strings.Join(strings.Fields(strings.Map(nameRune, raw)), " ")
	if name == "" {
		return "", invalidClientMetadata(fmt.Sprintf("client_name ausente ou vazio; esperado nome com 1 a %d caracteres", maxClientNameLen))
	}
	if count := utf8.RuneCountInString(name); count > maxClientNameLen {
		return "", invalidClientMetadata(fmt.Sprintf("client_name com %d caracteres; esperado no maximo %d", count, maxClientNameLen))
	}
	return name, nil
}

func nameRune(r rune) rune {
	switch {
	case unicode.IsControl(r):
		return ' '
	case unicode.Is(unicode.Cf, r):
		return -1
	}
	return r
}

// checkDCRRedirectURIs aplica a mesma regra das redirect_uris do documento CIMD.
func checkDCRRedirectURIs(uris []string) error {
	if len(uris) == 0 {
		return &ProtocolError{Code: errInvalidRedirectURI, Description: "redirect_uris ausente ou vazio; esperada lista com ao menos uma URI"}
	}
	for _, raw := range uris {
		if reason := cimd.RedirectURIProblem(raw); reason != "" {
			return &ProtocolError{Code: errInvalidRedirectURI, Description: "redirect_uri " + ClientValue(raw) + " " + reason + "; esperado " + cimd.RedirectURIShape}
		}
	}
	return nil
}

func invalidClientMetadata(description string) *ProtocolError {
	return &ProtocolError{Code: errInvalidClientMetadata, Description: description}
}

// registrationFailure traduz o teto atingido na recusa temporária, com ao menos um
// segundo de espera, e embrulha o resto como falha interna.
func registrationFailure(err error, now time.Time) error {
	var limit *PendingClientLimitError
	if !errors.As(err, &limit) {
		return fmt.Errorf("oauth: registrando cliente dcr: %w", err)
	}
	refusal := &ProtocolError{
		Code: errTemporarilyUnavailable, RetryAfter: max(limit.FreesAt.Sub(now), time.Second),
		Description: "registro dinamico de clientes lotado; esperada nova tentativa depois do Retry-After",
	}
	return refusal.WithInternalCause(err)
}

func registrationResponse(client Client, issuedAt time.Time) RegistrationResponse {
	return RegistrationResponse{
		ClientID: client.ID, ClientIDIssuedAt: issuedAt.Unix(), ClientName: client.Name,
		RedirectURIs: client.RedirectURIs, TokenEndpointAuthMethod: "none",
		GrantTypes: dcrGrantTypes(), ResponseTypes: []string{"code"},
	}
}
