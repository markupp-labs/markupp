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
// a URL pública do servidor, sem caminho, base dos endpoints que as próximas fatias anunciarem.
type OAuthMetadata struct {
	Issuer string
}

type oauthMetadataResponse struct {
	Issuer          string   `json:"issuer"`
	ScopesSupported []string `json:"scopes_supported"`
}

func newOAuthMetadataResponse(issuer string) oauthMetadataResponse {
	return oauthMetadataResponse{Issuer: issuer, ScopesSupported: oauth.SupportedScopes()}
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
