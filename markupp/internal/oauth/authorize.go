package oauth

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// StartAuthorization valida o GET /oauth/authorize, grava o Pedido de autorização e
// devolve a URL da tela de consentimento no painel. Enquanto cliente e redirect_uri não
// estão confirmados, o erro não leva RedirectURI; depois, volta ao cliente por redirect.
func (s *Service) StartAuthorization(ctx context.Context, p AuthorizeParams) (string, error) {
	if err := s.checkClientRedirect(ctx, p); err != nil {
		return "", err
	}
	req, err := s.buildRequest(p)
	if err != nil {
		return "", err
	}
	if err := s.repo.CreateRequest(ctx, req); err != nil {
		return "", fmt.Errorf("oauth: gravando o pedido de autorização do cliente %s: %w", ClientValue(p.ClientID), err)
	}
	return s.cfg.PanelURL + "/oauth/consentimento?pedido=" + url.QueryEscape(req.ID), nil
}

// registeredClientShape descreve o client_id aceito, para as mensagens de erro.
const registeredClientShape = "um cliente registrado por CIMD ou DCR"

// checkClientRedirect resolve o cliente e confirma que ele registrou a redirect_uri pedida.
func (s *Service) checkClientRedirect(ctx context.Context, p AuthorizeParams) error {
	if p.ClientID == "" {
		return &ProtocolError{Code: errInvalidRequest, Description: "client_id ausente; esperado " + registeredClientShape}
	}
	client, err := s.resolveClient(ctx, p.ClientID)
	if err != nil {
		return err
	}
	if !MatchRedirectURI(client.RedirectURIs, p.RedirectURI) {
		return &ProtocolError{Code: errInvalidRequest, Description: "redirect_uri " + ClientValue(p.RedirectURI) + " nao registrada para o cliente; esperada uma das redirect_uris registradas por ele"}
	}
	return nil
}

// buildRequest valida o resto dos parâmetros e monta o pedido a gravar.
func (s *Service) buildRequest(p AuthorizeParams) (AuthorizationRequest, error) {
	if err := checkCodeFlow(p); err != nil {
		return AuthorizationRequest{}, err
	}
	resource, err := s.requestedResource(p)
	if err != nil {
		return AuthorizationRequest{}, err
	}
	scopes, err := requestedScopes(p)
	if err != nil {
		return AuthorizationRequest{}, err
	}
	now := s.clock()
	return AuthorizationRequest{
		ID: s.newID(), ClientID: p.ClientID, RedirectURI: p.RedirectURI, Scopes: scopes,
		Resource: resource, State: p.State, CodeChallenge: p.CodeChallenge,
		CreatedAt: now, ExpiresAt: now.Add(s.cfg.RequestTTL),
	}, nil
}

// checkCodeFlow exige response_type=code e PKCE com S256.
func checkCodeFlow(p AuthorizeParams) error {
	if p.ResponseType != "code" {
		return redirectError(p, errUnsupportedResponseType, "response_type "+ClientValue(p.ResponseType)+" nao suportado; esperado 'code'")
	}
	if p.CodeChallengeMethod != "S256" {
		return redirectError(p, errInvalidRequest, "code_challenge_method "+ClientValue(p.CodeChallengeMethod)+" nao suportado; esperado 'S256'")
	}
	if !ValidChallenge(p.CodeChallenge) {
		return redirectError(p, errInvalidRequest, "code_challenge invalido; esperado SHA-256 em base64url sem padding, 43 caracteres")
	}
	return nil
}

// requestedResource devolve o resource na forma canônica, se for um servidor protegido.
func (s *Service) requestedResource(p AuthorizeParams) (string, error) {
	if p.Resource == "" {
		return "", redirectError(p, errInvalidTarget, "resource ausente; esperado "+s.knownResources())
	}
	resource, ok := canonicalize(p.Resource)
	if !ok || !slices.Contains(s.cfg.Resources, resource) {
		return "", redirectError(p, errInvalidTarget, "resource "+ClientValue(p.Resource)+" desconhecido; esperado "+s.knownResources())
	}
	return resource, nil
}

// knownResources lista os servidores protegidos aceitos como resource, no conjunto de
// caracteres do error_description. Sem nenhum configurado, diz isso em vez de listar.
func (s *Service) knownResources() string {
	if len(s.cfg.Resources) == 0 {
		return "um servidor protegido, mas nao ha nenhum servidor protegido configurado"
	}
	described := make([]string, 0, len(s.cfg.Resources))
	for _, resource := range s.cfg.Resources {
		described = append(described, ClientValue(resource))
	}
	return "um de " + strings.Join(described, ", ")
}

// requestedScopes separa o scope por espaço, recusa escopo fora de SupportedScopes e
// devolve os pedidos sem repetição, na ordem de SupportedScopes. Vazio vira leitura.
func requestedScopes(p AuthorizeParams) ([]string, error) {
	requested := map[string]bool{}
	for _, scope := range strings.Split(p.Scope, " ") {
		if scope != "" && !slices.Contains(SupportedScopes(), scope) {
			return nil, redirectError(p, errInvalidScope, "escopo "+ClientValue(scope)+" nao suportado; esperado um de "+strings.Join(SupportedScopes(), ", "))
		}
		requested[scope] = scope != ""
	}
	scopes := slices.DeleteFunc(SupportedScopes(), func(scope string) bool { return !requested[scope] })
	if len(scopes) == 0 {
		return []string{ScopeNotesRead}, nil
	}
	return scopes, nil
}

// redirectError é o erro que volta ao cliente pela redirect_uri já confirmada.
func redirectError(p AuthorizeParams, code, description string) error {
	return &ProtocolError{Code: code, Description: description, RedirectURI: p.RedirectURI, State: p.State}
}
