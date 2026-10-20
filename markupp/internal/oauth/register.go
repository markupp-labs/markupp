package oauth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

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
	name, err := cimd.SanitizeClientName(req.ClientName)
	if err != nil {
		return Client{}, invalidClientMetadata(err.Error())
	}
	if err := checkDCRRedirectURIs(req.RedirectURIs); err != nil {
		return Client{}, err
	}
	return Client{Name: name, RedirectURIs: req.RedirectURIs, Source: ClientSourceDCR}, nil
}

// checkPublicClientMetadata aceita só o cliente público do fluxo authorization code. Lista
// presente e vazia é recusada, como em grant_types e response_types: ausente vale o padrão.
func checkPublicClientMetadata(req RegistrationRequest) error {
	switch {
	case presentButEmpty(req.GrantTypes):
		return invalidClientMetadata("grant_types presente e vazio; esperado ausente ou com ao menos um valor")
	case presentButEmpty(req.ResponseTypes):
		return invalidClientMetadata("response_types presente e vazio; esperado ausente ou com ao menos um valor")
	case req.TokenEndpointAuthMethod != nil && *req.TokenEndpointAuthMethod != "none":
		return invalidClientMetadata("token_endpoint_auth_method " + ClientValue(*req.TokenEndpointAuthMethod) + "; esperado ausente ou none")
	case slices.ContainsFunc(req.GrantTypes, func(g string) bool { return !slices.Contains(dcrGrantTypes(), g) }):
		return invalidClientMetadata("grant_types " + ClientValue(strings.Join(req.GrantTypes, " ")) + "; esperado ausente ou subconjunto de authorization_code e refresh_token")
	case req.ResponseTypes != nil && !slices.Equal(req.ResponseTypes, []string{"code"}):
		return invalidClientMetadata("response_types " + ClientValue(strings.Join(req.ResponseTypes, " ")) + "; esperado ausente ou so code")
	}
	return nil
}

// presentButEmpty diz se a lista veio no corpo, mesmo como [], sem nenhum valor.
func presentButEmpty(list []string) bool {
	return list != nil && len(list) == 0
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
