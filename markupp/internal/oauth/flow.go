package oauth

import (
	"fmt"
	"net/url"
)

// AuthorizeParams são os parâmetros de GET /oauth/authorize.
type AuthorizeParams struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resource            string
}

// RequestView é o que o painel mostra na tela de consentimento.
type RequestView struct {
	ID                string
	ClientID          string
	ClientName        string
	RedirectHost      string
	Scopes            []string
	AlreadyAuthorized bool
	ExpiresAt         string
}

// CodeGrant são os parâmetros de POST /oauth/token com grant_type=authorization_code.
type CodeGrant struct {
	Code         string
	CodeVerifier string
	ClientID     string
	RedirectURI  string
	Resource     string
}

// TokenResponse é a resposta de sucesso do endpoint de token (RFC 6749, seção 5.1).
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// ProtocolError é um erro do protocolo OAuth com o código da RFC 6749 (invalid_request,
// invalid_scope, invalid_grant, access_denied, invalid_target...). Quando RedirectURI vem
// preenchida, o erro volta ao cliente por redirect com State; senão, é mostrado ali mesmo.
type ProtocolError struct {
	Code        string
	Description string
	RedirectURI string
	State       string
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("oauth %s: %s", e.Code, e.Description)
}

// RedirectLocation monta o redirect que devolve o erro ao cliente, com o issuer no
// parâmetro iss (RFC 9207).
func (e *ProtocolError) RedirectLocation(issuer string) string {
	return AppendQuery(e.RedirectURI, map[string]string{
		"error": e.Code, "error_description": e.Description, "state": e.State, "iss": issuer,
	})
}

// AppendQuery acrescenta à URI os parâmetros não vazios, preservando a query que ela já
// tem. Uma URI ilegível volta sem mudança, porque só chega aqui depois de validada.
func AppendQuery(uri string, params map[string]string) string {
	parsed, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	query := parsed.Query()
	for key, value := range params {
		if value != "" {
			query.Set(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
