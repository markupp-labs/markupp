package loginprovider_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

func TestNewRegistry_ListaNaOrdemDaConfiguracao(t *testing.T) {
	registry := loginprovider.NewRegistry([]loginprovider.Settings{
		{ID: "google", Name: "Google", Issuer: "https://accounts.google.com", ClientID: "a", ClientSecret: "b"},
		{ID: "github", Name: "GitHub", ClientID: "c", ClientSecret: "d"},
	}, http.DefaultClient)

	providers := registry.List()

	require.Len(t, providers, 2)
	assert.Equal(t, "google", providers[0].ID())
	assert.Equal(t, "GitHub", providers[1].Name())
}

func TestRegistry_Get_DevolveOProvedorPeloId(t *testing.T) {
	registry := loginprovider.NewRegistry([]loginprovider.Settings{{ID: "github", Name: "GitHub"}}, http.DefaultClient)

	provider, ok := registry.Get("github")

	require.True(t, ok)
	assert.Equal(t, "github", provider.ID())
}

func TestRegistry_Get_IdDesconhecido_NaoAcha(t *testing.T) {
	registry := loginprovider.NewRegistry(nil, http.DefaultClient)

	_, ok := registry.Get("github")

	assert.False(t, ok)
}
