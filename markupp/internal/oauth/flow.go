package oauth

import (
	"fmt"
	"net/url"
	"time"
)

// Identificadores da troca de token (RFC 8693). São URNs fixas da especificação, não
// credenciais, por isso o gosec é silenciado nelas.
const (
	GrantTypeTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange" //nolint:gosec // URN da RFC 8693
	TokenTypeAccessToken   = "urn:ietf:params:oauth:token-type:access_token"   //nolint:gosec // URN da RFC 8693
)

// ExchangeGrant são os parâmetros de POST /oauth/token com o grant de troca de token. O
// resource se autentica por HTTP Basic: ResourceID é a URL dele e ResourceSecret o
// segredo configurado.
type ExchangeGrant struct {
	ResourceID         string
	ResourceSecret     string
	SubjectToken       string
	SubjectTokenType   string
	RequestedTokenType string
	Audience           string
	Resource           string
}

// ExchangeResponse é a resposta de sucesso da troca de token (RFC 8693, seção 2.2.1).
type ExchangeResponse struct {
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int64  `json:"expires_in"`
	Scope           string `json:"scope"`
}

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
	ID         string
	ClientID   string
	ClientName string
	// ClientSource é a origem do cliente, para o painel marcar o nome do cliente DCR
	// como não verificado.
	ClientSource string
	// RedirectHost é o host da redirect_uri; no esquema privado, precedido do esquema
	// (como "evilapp://claude.ai") ou, sem host, o esquema com dois-pontos ("cursor:").
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

// RefreshGrant são os parâmetros de POST /oauth/token com grant_type=refresh_token.
type RefreshGrant struct {
	RefreshToken string
	ClientID     string
	Scope        string
	Resource     string
}

// RevokeRequest são os parâmetros de POST /oauth/revoke (RFC 7009).
type RevokeRequest struct {
	Token    string
	ClientID string
}

// AuthorizationView é a Autorização ativa como a conta a vê, com datas em RFC 3339 UTC.
type AuthorizationView struct {
	ID         string   `json:"id"`
	ClientID   string   `json:"client_id"`
	ClientName string   `json:"client_name"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

// TokenResponse é a resposta de sucesso do endpoint de token (RFC 6749, seção 5.1).
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// RegistrationRequest é o corpo de POST /oauth/register (RFC 7591, seção 2), com os
// campos que o servidor confere. Lista ausente fica nil e o método de autenticação
// ausente fica nil; os campos desconhecidos são ignorados.
type RegistrationRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod *string  `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
}

// RegistrationResponse é a resposta de sucesso do registro dinâmico (RFC 7591, seção
// 3.2.1): um cliente público, sem client_secret, com os valores que o servidor aplica.
type RegistrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
}

// ProtocolError é um erro do protocolo OAuth com o código da RFC 6749 (invalid_request,
// invalid_scope, invalid_grant, access_denied, invalid_target...). Quando RedirectURI vem
// preenchida, o erro volta ao cliente por redirect com State; senão, é mostrado ali mesmo.
// A causa interna, quando há, serve só para o log e nunca entra em Error nem na resposta.
// RetryAfter positivo marca a recusa temporária, que o cliente pode repetir depois dele.
type ProtocolError struct {
	Code        string
	Description string
	RedirectURI string
	State       string
	RetryAfter  time.Duration
	cause       error
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("oauth %s: %s", e.Code, e.Description)
}

// WithInternalCause devolve uma cópia do erro com a causa que só vai para o log.
func (e *ProtocolError) WithInternalCause(cause error) *ProtocolError {
	withCause := *e
	withCause.cause = cause
	return &withCause
}

// Unwrap devolve a causa interna, ou nil.
func (e *ProtocolError) Unwrap() error {
	return e.cause
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
