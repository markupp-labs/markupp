package oauth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func TestSupportedScopes_OrdemFixa_ListaOsTresEscopos(t *testing.T) {
	assert.Equal(t, []string{"notas:leitura", "notas:escrita", "administracao"}, oauth.SupportedScopes())
}

func TestSupportedScopes_AlteracaoDoChamador_NaoAfetaAProximaChamada(t *testing.T) {
	scopes := oauth.SupportedScopes()
	scopes[0] = "outro"

	assert.Equal(t, oauth.ScopeNotesRead, oauth.SupportedScopes()[0])
}
