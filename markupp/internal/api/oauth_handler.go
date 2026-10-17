package api

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// OAuthService é o authorization server OAuth consumido pelas rotas HTTP.
type OAuthService interface {
	StartAuthorization(ctx context.Context, p oauth.AuthorizeParams) (consentURL string, err error)
	DescribeRequest(ctx context.Context, requestID, userID string) (oauth.RequestView, error)
	Approve(ctx context.Context, requestID, userID string) (redirectTo string, err error)
	Deny(ctx context.Context, requestID, userID string) (redirectTo string, err error)
	ExchangeCode(ctx context.Context, g oauth.CodeGrant) (oauth.TokenResponse, error)
}

const (
	grantTypeAuthorizationCode = "authorization_code"
	formURLEncoded             = "application/x-www-form-urlencoded"
)

type oauthHandler struct {
	svc    OAuthService
	issuer string
}

// oauthErrorResponse é o corpo de erro do protocolo (RFC 6749, seção 5.2), diferente do
// errorResponse do resto da API.
type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// mountOAuthServerRoutes monta na raiz, fora das rotas versionadas, a metadata e os
// endpoints de autorização e de token que os clientes OAuth acessam direto. Os
// endpoints ficam num subroteador próprio para que método não suportado responda 405
// ali mesmo, sem cair no alias obsoleto sem prefixo.
func mountOAuthServerRoutes(r chi.Router, d RouterDeps) {
	if d.OAuthMetadata == nil {
		return
	}
	r.Get(oauthMetadataPath, oauthMetadataHandler(*d.OAuthMetadata))
	if d.OAuth == nil {
		return
	}
	h := &oauthHandler{svc: d.OAuth, issuer: d.OAuthMetadata.Issuer}
	r.Route("/oauth", func(p chi.Router) {
		p.Get("/authorize", h.authorize)
		p.Post("/token", h.token)
		p.Options("/token", h.tokenPreflight)
	})
}

func (h *oauthHandler) authorize(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if protoErr := rejectRepeatedParam(query); protoErr != nil {
		writeOAuthError(w, protoErr)
		return
	}
	consentURL, err := h.svc.StartAuthorization(r.Context(), authorizeParamsFrom(query))
	if err != nil {
		h.writeAuthorizeError(w, r, err)
		return
	}
	http.Redirect(w, r, consentURL, http.StatusFound) //nolint:gosec // G710: o serviço monta o destino sobre a URL do painel, não a partir da query
}

func authorizeParamsFrom(query url.Values) oauth.AuthorizeParams {
	return oauth.AuthorizeParams{
		ResponseType:        query.Get("response_type"),
		ClientID:            query.Get("client_id"),
		RedirectURI:         query.Get("redirect_uri"),
		Scope:               query.Get("scope"),
		State:               query.Get("state"),
		CodeChallenge:       query.Get("code_challenge"),
		CodeChallengeMethod: query.Get("code_challenge_method"),
		Resource:            query.Get("resource"),
	}
}

// writeAuthorizeError devolve o erro ao cliente por redirect só quando o serviço já
// validou a redirect_uri; sem ela, o erro é mostrado ali mesmo.
func (h *oauthHandler) writeAuthorizeError(w http.ResponseWriter, r *http.Request, err error) {
	var protoErr *oauth.ProtocolError
	if !errors.As(err, &protoErr) {
		writeInternal(w, "iniciar autorizacao oauth", err)
		return
	}
	if protoErr.RedirectURI == "" {
		writeOAuthError(w, protoErr)
		return
	}
	http.Redirect(w, r, protoErr.RedirectLocation(h.issuer), http.StatusFound)
}

// token atende o endpoint de token, chamado por clientes de qualquer origem: CORS
// aberto sem credenciais e nenhuma resposta guardada em cache.
func (h *oauthHandler) token(w http.ResponseWriter, r *http.Request) {
	allowAnyOrigin(w)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	form, protoErr := readTokenForm(w, r)
	if protoErr != nil {
		writeOAuthError(w, protoErr)
		return
	}
	tokens, err := h.svc.ExchangeCode(r.Context(), codeGrantFrom(form))
	if err != nil {
		writeTokenError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (h *oauthHandler) tokenPreflight(w http.ResponseWriter, _ *http.Request) {
	allowAnyOrigin(w)
	w.Header().Set("Access-Control-Allow-Methods", http.MethodPost+", "+http.MethodOptions)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", preflightMaxAge)
	w.WriteHeader(http.StatusNoContent)
}

// readTokenForm lê só o corpo do formulário, sem a query, e recusa com o erro do
// protocolo o que não for formulário, parâmetro repetido e grant_type não suportado.
func readTokenForm(w http.ResponseWriter, r *http.Request) (url.Values, *oauth.ProtocolError) {
	contentType := r.Header.Get("Content-Type")
	if mediaType, _, _ := mime.ParseMediaType(contentType); mediaType != formURLEncoded {
		return nil, invalidRequest(fmt.Sprintf("corpo deve ser %s, recebido Content-Type %q", formURLEncoded, contentType))
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	if err := r.ParseForm(); err != nil {
		return nil, invalidRequest(fmt.Sprintf("formulario ilegivel ou acima de %d bytes", maxAuthBodyBytes))
	}
	if protoErr := rejectRepeatedParam(r.PostForm); protoErr != nil {
		return nil, protoErr
	}
	if grantType := r.PostForm.Get("grant_type"); grantType != grantTypeAuthorizationCode {
		return nil, &oauth.ProtocolError{Code: "unsupported_grant_type", Description: fmt.Sprintf("grant_type deve ser %s, recebido %q", grantTypeAuthorizationCode, grantType)}
	}
	return r.PostForm, nil
}

func codeGrantFrom(form url.Values) oauth.CodeGrant {
	return oauth.CodeGrant{
		Code:         form.Get("code"),
		CodeVerifier: form.Get("code_verifier"),
		ClientID:     form.Get("client_id"),
		RedirectURI:  form.Get("redirect_uri"),
		Resource:     form.Get("resource"),
	}
}

func writeTokenError(w http.ResponseWriter, err error) {
	var protoErr *oauth.ProtocolError
	if errors.As(err, &protoErr) {
		writeOAuthError(w, protoErr)
		return
	}
	writeInternal(w, "trocar codigo oauth", err)
}

// rejectRepeatedParam recusa parâmetro enviado mais de uma vez (RFC 6749, seção 3.1),
// nomeando o primeiro encontrado sem ecoar os valores.
func rejectRepeatedParam(values url.Values) *oauth.ProtocolError {
	for name, list := range values {
		if len(list) > 1 {
			return invalidRequest(fmt.Sprintf("parametro %q deve aparecer uma vez, recebido %d vezes", name, len(list)))
		}
	}
	return nil
}

func invalidRequest(description string) *oauth.ProtocolError {
	return &oauth.ProtocolError{Code: "invalid_request", Description: description}
}

func writeOAuthError(w http.ResponseWriter, e *oauth.ProtocolError) {
	writeJSON(w, http.StatusBadRequest, oauthErrorResponse{Error: e.Code, ErrorDescription: e.Description})
}
