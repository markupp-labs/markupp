package api

import "github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"

// PanelRelayID é o usuário do HTTP Basic com que o painel se apresenta como relay da
// leitura pública. Não é URL, então nunca coincide com um resource do OAuth.
const PanelRelayID = "panel"

// PanelRelay confere o painel pelo segredo de relay que o servidor gera só para ele, e
// vale com o MCP e o OAuth desligados.
type PanelRelay struct {
	Secret string
}

// AuthenticatesResource aceita só o usuário do painel com o segredo dele; sem segredo
// configurado, não aceita ninguém.
func (p PanelRelay) AuthenticatesResource(resourceID, secret string) bool {
	return p.Secret != "" && resourceID == PanelRelayID && auth.SameSecret(secret, p.Secret)
}

// RelayAuthenticators aceita o relay que qualquer um deles aceita.
type RelayAuthenticators []RelayAuthenticator

// AuthenticatesResource pergunta a cada relay, na ordem.
func (r RelayAuthenticators) AuthenticatesResource(resourceID, secret string) bool {
	for _, relay := range r {
		if relay.AuthenticatesResource(resourceID, secret) {
			return true
		}
	}
	return false
}
