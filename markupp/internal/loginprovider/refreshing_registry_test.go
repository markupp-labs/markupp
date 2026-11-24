package loginprovider_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

// FakeLoginProviderSource devolve a lista e a revisão que o teste põe, e conta as leituras.
type FakeLoginProviderSource struct {
	revision string
	settings []loginprovider.Settings
	err      error
	reads    int
}

func (f *FakeLoginProviderSource) LoginProviders(context.Context) (string, []loginprovider.Settings, error) {
	f.reads++
	return f.revision, f.settings, f.err
}

// FakeClock é o relógio que o teste avança.
type FakeClock struct{ at time.Time }

func (c *FakeClock) Now() time.Time { return c.at }

func registroQueSeAtualiza(source *FakeLoginProviderSource, clock *FakeClock) *loginprovider.RefreshingRegistry {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return loginprovider.NewRefreshingRegistry(source, http.DefaultClient, 5*time.Second, clock.Now, logger)
}

func githubDeTeste(name string) []loginprovider.Settings {
	return []loginprovider.Settings{{ID: "github", Name: name, ClientID: "id", ClientSecret: "s"}}
}

func TestRefreshingRegistry_PrimeiraLeitura_ListaOsProvedores(t *testing.T) {
	source := &FakeLoginProviderSource{revision: "r1", settings: githubDeTeste("GitHub")}
	registry := registroQueSeAtualiza(source, &FakeClock{at: time.Unix(0, 0)})

	p, ok := registry.Get("github")

	require.True(t, ok)
	assert.Equal(t, "GitHub", p.Name())
	assert.Len(t, registry.List(), 1)
}

func TestRefreshingRegistry_DentroDoIntervalo_NaoReleAFonte(t *testing.T) {
	source := &FakeLoginProviderSource{revision: "r1", settings: githubDeTeste("GitHub")}
	clock := &FakeClock{at: time.Unix(0, 0)}
	registry := registroQueSeAtualiza(source, clock)
	registry.List()

	clock.at = clock.at.Add(4 * time.Second)
	registry.List()

	assert.Equal(t, 1, source.reads)
}

func TestRefreshingRegistry_RevisaoNova_TrocaOsProvedores(t *testing.T) {
	source := &FakeLoginProviderSource{revision: "r1", settings: githubDeTeste("GitHub")}
	clock := &FakeClock{at: time.Unix(0, 0)}
	registry := registroQueSeAtualiza(source, clock)
	registry.List()

	source.revision, source.settings = "r2", githubDeTeste("GitHub Enterprise")
	clock.at = clock.at.Add(6 * time.Second)
	p, ok := registry.Get("github")

	require.True(t, ok)
	assert.Equal(t, "GitHub Enterprise", p.Name())
}

// O provedor sem mudança guarda a descoberta OIDC já feita.
func TestRefreshingRegistry_RevisaoNovaComProvedorIgual_MantemOMesmoAdaptador(t *testing.T) {
	source := &FakeLoginProviderSource{revision: "r1", settings: githubDeTeste("GitHub")}
	clock := &FakeClock{at: time.Unix(0, 0)}
	registry := registroQueSeAtualiza(source, clock)
	before, _ := registry.Get("github")

	source.revision = "r2"
	source.settings = append(githubDeTeste("GitHub"), loginprovider.Settings{ID: "google", Issuer: "https://accounts.google.com", ClientID: "id", ClientSecret: "s"})
	clock.at = clock.at.Add(6 * time.Second)
	after, _ := registry.Get("github")

	assert.Same(t, before, after)
	assert.Len(t, registry.List(), 2)
}

func TestRefreshingRegistry_FonteFora_MantemAUltimaLista(t *testing.T) {
	source := &FakeLoginProviderSource{revision: "r1", settings: githubDeTeste("GitHub")}
	clock := &FakeClock{at: time.Unix(0, 0)}
	registry := registroQueSeAtualiza(source, clock)
	registry.List()

	source.err = errors.New("banco fora")
	clock.at = clock.at.Add(6 * time.Second)

	assert.Len(t, registry.List(), 1)
}

func TestRefreshingRegistry_FonteForaNaPrimeiraLeitura_ListaVazia(t *testing.T) {
	source := &FakeLoginProviderSource{err: errors.New("banco fora")}
	registry := registroQueSeAtualiza(source, &FakeClock{at: time.Unix(0, 0)})

	_, ok := registry.Get("github")

	assert.False(t, ok)
	assert.Empty(t, registry.List())
}
