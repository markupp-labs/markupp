package oauth_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func TestAppendQuery_URIComQuery_PreservaEAcrescenta(t *testing.T) {
	got := oauth.AppendQuery("http://127.0.0.1:4000/cb?x=1", map[string]string{"code": "abc", "state": ""})

	assert.Equal(t, "http://127.0.0.1:4000/cb?code=abc&x=1", got)
}

func TestProtocolErrorRedirectLocation_ComState_LevaErroStateEIssuer(t *testing.T) {
	err := &oauth.ProtocolError{Code: "invalid_scope", Description: "escopo desconhecido", RedirectURI: "https://app.dev/cb", State: "s1"}

	got := err.RedirectLocation("https://notas.markupp.dev")

	assert.Equal(t, "https://app.dev/cb?error=invalid_scope&error_description=escopo+desconhecido&iss=https%3A%2F%2Fnotas.markupp.dev&state=s1", got)
}

func TestProtocolErrorWithInternalCause_GuardaACausaForaDaMensagem(t *testing.T) {
	cause := errors.New("dial tcp 10.0.0.7:443: connection refused")
	original := &oauth.ProtocolError{Code: "invalid_request", Description: "documento invalido"}

	err := original.WithInternalCause(cause)

	assert.ErrorIs(t, err, cause)
	assert.Equal(t, "oauth invalid_request: documento invalido", err.Error())
	assert.NoError(t, original.Unwrap(), "o original fica sem causa")
}
