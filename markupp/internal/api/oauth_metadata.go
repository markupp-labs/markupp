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
// a URL pública do servidor, sem caminho, base de todos os endpoints anunciados.
type OAuthMetadata struct {
	Issuer string
}

type oauthMetadataResponse struct {
	Issuer                                 string   `json:"issuer"`
	AuthorizationEndpoint                  string   `json:"authorization_endpoint"`
	TokenEndpoint                          string   `json:"token_endpoint"`
	RegistrationEndpoint                   string   `json:"registration_endpoint"`
	RevocationEndpoint                     string   `json:"revocation_endpoint"`
	ResponseTypesSupported                 []string `json:"response_types_supported"`
	GrantTypesSupported                    []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported          []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported      []string `json:"token_endpoint_auth_methods_supported"`
	RevocationEndpointAuthMethodsSupported []string `json:"revocation_endpoint_auth_methods_supported"`
	ScopesSupported                        []string `json:"scopes_supported"`
	ClientIDMetadataDocumentSupported      bool     `json:"client_id_metadata_document_supported"`
}

func newOAuthMetadataResponse(issuer string) oauthMetadataResponse {
	return oauthMetadataResponse{
		Issuer:                                 issuer,
		AuthorizationEndpoint:                  issuer + "/oauth/authorize",
		TokenEndpoint:                          issuer + "/oauth/token",
		RegistrationEndpoint:                   issuer + "/oauth/register",
		RevocationEndpoint:                     issuer + "/oauth/revoke",
		ResponseTypesSupported:                 []string{"code"},
		GrantTypesSupported:                    []string{"authorization_code", "refresh_token", "urn:ietf:params:oauth:grant-type:token-exchange"},
		CodeChallengeMethodsSupported:          []string{"S256"},
		TokenEndpointAuthMethodsSupported:      []string{"none", "client_secret_basic"},
		RevocationEndpointAuthMethodsSupported: []string{"none"},
		ScopesSupported:                        oauth.SupportedScopes(),
		ClientIDMetadataDocumentSupported:      true,
	}
}

// oauthMetadataHandler serve o documento de metadados, montado uma vez e reaproveitado.
// O documento é público e lido por clientes de qualquer origem, então libera CORS
// para todas, sem credenciais, independente da lista de origens do servidor.
func oauthMetadataHandler(m OAuthMetadata) http.HandlerFunc {
	mustBeAbsoluteIssuer(m.Issuer)
	body := newOAuthMetadataResponse(m.Issuer)
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
