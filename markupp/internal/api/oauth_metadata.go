package api

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// oauthMetadataPath é onde o authorization server publica seus metadados (RFC 8414),
// na raiz e fora das rotas versionadas.
const oauthMetadataPath = "/.well-known/oauth-authorization-server"

// OAuthMetadata configura o documento de metadados do authorization server. Issuer é
// a URL pública do servidor, sem caminho, base dos endpoints anunciados.
type OAuthMetadata struct {
	Issuer string
}

// Caminhos dos endpoints do protocolo, anunciados na metadata sobre o Issuer.
const (
	oauthAuthorizePath = "/oauth/authorize"
	oauthTokenPath     = "/oauth/token" //nolint:gosec // G101: caminho do endpoint, não credencial
)

type oauthMetadataResponse struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	ScopesSupported                   []string `json:"scopes_supported"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// newOAuthMetadataResponse anuncia o fluxo de código com PKCE S256 e cliente público,
// sem segredo no endpoint de token.
func newOAuthMetadataResponse(issuer string) oauthMetadataResponse {
	return oauthMetadataResponse{
		Issuer:                            issuer,
		AuthorizationEndpoint:             issuer + oauthAuthorizePath,
		TokenEndpoint:                     issuer + oauthTokenPath,
		ScopesSupported:                   oauth.SupportedScopes(),
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{grantTypeAuthorizationCode},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
	}
}

// oauthMetadataHandler serve o documento de metadados, montado uma vez e reaproveitado.
// O documento é público e lido por clientes de qualquer origem, que o subroteador
// libera com CORS aberto.
func oauthMetadataHandler(m OAuthMetadata) http.HandlerFunc {
	mustBeAbsoluteIssuer(m.Issuer)
	body := newOAuthMetadataResponse(m.Issuer)
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, body)
	}
}

// mustBeAbsoluteIssuer barra na montagem do roteador um Issuer que geraria endpoints
// relativos ou quebrados no documento de metadados.
func mustBeAbsoluteIssuer(issuer string) {
	u, err := url.Parse(issuer)
	if err != nil || u.Scheme == "" || u.Host == "" {
		panic(fmt.Sprintf("api: OAuthMetadata.Issuer deve ser URL absoluta com esquema e host (ex.: https://markupp.exemplo), recebido %q", issuer))
	}
}
