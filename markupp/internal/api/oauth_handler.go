package api

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"slices"

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
	RefreshToken(ctx context.Context, g oauth.RefreshGrant) (oauth.TokenResponse, error)
	RevokeToken(ctx context.Context, r oauth.RevokeRequest) error
	ListAuthorizations(ctx context.Context, userID string) ([]oauth.AuthorizationView, error)
	RevokeAuthorization(ctx context.Context, userID, authorizationID string) error
}

const (
	grantTypeAuthorizationCode = "authorization_code"
	grantTypeRefreshToken      = "refresh_token"
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
// endpoints de autorização e de token que os clientes OAuth acessam direto. Cada um
// fica num subroteador próprio para que método não suportado responda 405 ali mesmo,
// sem cair no alias obsoleto sem prefixo nem no CORS da lista de origens.
func mountOAuthServerRoutes(r chi.Router, d RouterDeps) {
	if d.OAuthMetadata == nil {
		return
	}
	r.Route(oauthMetadataPath, func(m chi.Router) {
		m.Use(allowAnyOrigin)
		m.Get("/", oauthMetadataHandler(*d.OAuthMetadata))
		m.Options("/", openPreflight(http.MethodGet, "Content-Type, "+mcpProtocolVersionHeader))
	})
	if d.OAuth != nil {
		mountOAuthProtocolRoutes(r, &oauthHandler{svc: d.OAuth, issuer: d.OAuthMetadata.Issuer})
	}
}

// mountOAuthProtocolRoutes deixa o authorize, aberto por navegação, sem CORS, e o resto
// de /oauth com CORS aberto, inclusive no 404 e no 405.
func mountOAuthProtocolRoutes(r chi.Router, h *oauthHandler) {
	r.Route(oauthAuthorizePath, func(a chi.Router) {
		a.Get("/", h.authorize)
	})
	r.Route("/oauth", func(p chi.Router) {
		p.Use(allowAnyOrigin)
		p.Post("/token", h.token)
		p.Options("/token", openPreflight(http.MethodPost, "Content-Type"))
		p.Post("/revoke", h.revoke)
		p.Options("/revoke", openPreflight(http.MethodPost, "Content-Type"))
	})
}

func (h *oauthHandler) authorize(w http.ResponseWriter, r *http.Request) {
	query, protoErr := readAuthorizeQuery(r)
	if protoErr != nil {
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

// readAuthorizeQuery lê a query sem descartar o par malformado, que r.URL.Query
// ignoraria, e recusa parâmetro repetido.
func readAuthorizeQuery(r *http.Request) (url.Values, *oauth.ProtocolError) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, invalidRequest(fmt.Sprintf("query malformada (%s), esperado %s valido", oauth.ClientValue(err.Error()), formURLEncoded))
	}
	if protoErr := rejectRepeatedParam(query); protoErr != nil {
		return nil, protoErr
	}
	return query, nil
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
		writeOAuthInternal(w, "iniciar autorizacao oauth", err)
		return
	}
	if protoErr.RedirectURI == "" {
		writeOAuthError(w, protoErr)
		return
	}
	http.Redirect(w, r, protoErr.RedirectLocation(h.issuer), http.StatusFound)
}

// token atende o endpoint de token, chamado por clientes de qualquer origem com o CORS
// aberto do subroteador, sem nenhuma resposta guardada em cache.
func (h *oauthHandler) token(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	form, protoErr := readOAuthForm(w, r)
	if protoErr != nil {
		writeOAuthError(w, protoErr)
		return
	}
	tokens, err := h.grantTokens(r.Context(), form)
	if err != nil {
		writeOAuthServiceError(w, "emitir tokens oauth", err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

// grantTokens despacha pelo grant_type e recusa com unsupported_grant_type o que não
// for authorization_code nem refresh_token.
func (h *oauthHandler) grantTokens(ctx context.Context, form url.Values) (oauth.TokenResponse, error) {
	switch grantType := form.Get("grant_type"); grantType {
	case grantTypeAuthorizationCode:
		return h.svc.ExchangeCode(ctx, codeGrantFrom(form))
	case grantTypeRefreshToken:
		return h.svc.RefreshToken(ctx, refreshGrantFrom(form))
	default:
		return oauth.TokenResponse{}, &oauth.ProtocolError{Code: "unsupported_grant_type", Description: fmt.Sprintf(
			"grant_type deve ser %s ou %s, recebido %s", grantTypeAuthorizationCode, grantTypeRefreshToken, oauth.ClientValue(grantType))}
	}
}

// readOAuthForm lê só o corpo do formulário, sem a query, e recusa com o erro do
// protocolo o que não for formulário e parâmetro repetido.
func readOAuthForm(w http.ResponseWriter, r *http.Request) (url.Values, *oauth.ProtocolError) {
	contentType := r.Header.Get("Content-Type")
	if mediaType, _, _ := mime.ParseMediaType(contentType); mediaType != formURLEncoded {
		return nil, invalidRequest(fmt.Sprintf("corpo deve ser %s, recebido Content-Type %s", formURLEncoded, oauth.ClientValue(contentType)))
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	if err := r.ParseForm(); err != nil {
		return nil, invalidRequest(fmt.Sprintf("formulario ilegivel ou acima de %d bytes", maxAuthBodyBytes))
	}
	if protoErr := rejectRepeatedParam(r.PostForm); protoErr != nil {
		return nil, protoErr
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

// revoke atende a revogação (RFC 7009): token desconhecido também responde 200, sem
// corpo, para não revelar quais tokens existem; token_type_hint é ignorado. O serviço
// recusa token ou client_id ausente.
func (h *oauthHandler) revoke(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	form, protoErr := readOAuthForm(w, r)
	if protoErr != nil {
		writeOAuthError(w, protoErr)
		return
	}
	err := h.svc.RevokeToken(r.Context(), oauth.RevokeRequest{Token: form.Get("token"), ClientID: form.Get("client_id")})
	if err != nil {
		writeOAuthServiceError(w, "revogar token oauth", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func refreshGrantFrom(form url.Values) oauth.RefreshGrant {
	return oauth.RefreshGrant{
		RefreshToken: form.Get("refresh_token"),
		ClientID:     form.Get("client_id"),
		Scope:        form.Get("scope"),
		Resource:     form.Get("resource"),
	}
}

// writeOAuthServiceError responde 400 com o erro do protocolo e 500 com o resto.
func writeOAuthServiceError(w http.ResponseWriter, operation string, err error) {
	var protoErr *oauth.ProtocolError
	if errors.As(err, &protoErr) {
		writeOAuthError(w, protoErr)
		return
	}
	writeOAuthInternal(w, operation, err)
}

// rejectRepeatedParam recusa parâmetro enviado mais de uma vez (RFC 6749, seção 3.1),
// nomeando o primeiro em ordem alfabética, para a resposta não variar, sem ecoar os
// valores.
func rejectRepeatedParam(values url.Values) *oauth.ProtocolError {
	for _, name := range slices.Sorted(maps.Keys(values)) {
		if count := len(values[name]); count > 1 {
			return invalidRequest(fmt.Sprintf("parametro %s deve aparecer uma vez, recebido %d vezes", oauth.ClientValue(name), count))
		}
	}
	return nil
}

func invalidRequest(description string) *oauth.ProtocolError {
	return &oauth.ProtocolError{Code: "invalid_request", Description: description}
}

func writeOAuthInternal(w http.ResponseWriter, operation string, err error) {
	writeLoggedInternal(w, "falha no oauth", operation, err)
}

func writeOAuthError(w http.ResponseWriter, e *oauth.ProtocolError) {
	writeJSON(w, http.StatusBadRequest, oauthErrorResponse{Error: e.Code, ErrorDescription: e.Description})
}
