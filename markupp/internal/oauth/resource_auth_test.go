package oauth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthenticatesResource_SoOParDoResourceConfigurado(t *testing.T) {
	svc := newTestService(&fakeOAuthRepository{})

	assert.True(t, svc.AuthenticatesResource(testResource, testResourceSecret))
	assert.False(t, svc.AuthenticatesResource(testResource, testResourceSecret+"x"), "segredo errado")
	assert.False(t, svc.AuthenticatesResource("https://outro.exemplo.dev/mcp", testResourceSecret), "resource desconhecido")
	assert.False(t, svc.AuthenticatesResource("", ""), "credencial vazia")
}
