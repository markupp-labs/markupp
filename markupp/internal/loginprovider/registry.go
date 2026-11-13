package loginprovider

import "net/http"

// Registry são os provedores configurados, na ordem da configuração.
type Registry struct {
	providers []Provider
}

// NewRegistry monta o adaptador de cada provedor: OIDC com Issuer, GitHub sem.
func NewRegistry(settings []Settings, client *http.Client) *Registry {
	r := &Registry{}
	for _, s := range settings {
		if s.Issuer == "" {
			r.providers = append(r.providers, NewGitHub(s, DefaultGitHubEndpoints, client))
			continue
		}
		r.providers = append(r.providers, NewOIDC(s, client))
	}
	return r
}

// List devolve os provedores na ordem da configuração.
func (r *Registry) List() []Provider {
	return r.providers
}

// Get devolve o provedor de id, ou false sem ele.
func (r *Registry) Get(id string) (Provider, bool) {
	for _, p := range r.providers {
		if p.ID() == id {
			return p, true
		}
	}
	return nil, false
}
