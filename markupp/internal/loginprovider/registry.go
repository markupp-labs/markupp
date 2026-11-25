package loginprovider

import "net/http"

// Registry são os provedores configurados, na ordem da configuração.
type Registry struct {
	entries []registryEntry
}

// registryEntry guarda as Settings de que o adaptador nasceu, para reusá-lo quando elas
// não mudam e manter a descoberta OIDC já feita.
type registryEntry struct {
	settings Settings
	provider Provider
}

// NewRegistry monta o adaptador de cada provedor: OIDC com Issuer, GitHub sem.
func NewRegistry(settings []Settings, client *http.Client) *Registry {
	return (&Registry{}).rebuilt(settings, client)
}

// rebuilt monta o registro de settings, reusando o adaptador das Settings que não mudaram.
func (r *Registry) rebuilt(settings []Settings, client *http.Client) *Registry {
	next := &Registry{entries: make([]registryEntry, 0, len(settings))}
	for _, s := range settings {
		next.entries = append(next.entries, registryEntry{settings: s, provider: r.reuse(s, client)})
	}
	return next
}

func (r *Registry) reuse(s Settings, client *http.Client) Provider {
	for _, e := range r.entries {
		if e.settings == s {
			return e.provider
		}
	}
	if s.Issuer == "" {
		return NewGitHub(s, DefaultGitHubEndpoints, client)
	}
	return NewOIDC(s, client)
}

// List devolve os provedores na ordem da configuração.
func (r *Registry) List() []Provider {
	providers := make([]Provider, 0, len(r.entries))
	for _, e := range r.entries {
		providers = append(providers, e.provider)
	}
	return providers
}

// Get devolve o provedor de id, ou false sem ele.
func (r *Registry) Get(id string) (Provider, bool) {
	for _, e := range r.entries {
		if e.provider.ID() == id {
			return e.provider, true
		}
	}
	return nil, false
}
