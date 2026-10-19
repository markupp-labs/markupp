package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// RouterDeps reúne o que o roteador expõe. As rotas de um serviço nil não são montadas.
type RouterDeps struct {
	Notes          NoteService
	Auth           AuthService
	Admin          AdminService
	AccountEmail   AccountEmailService
	Tokens         TokenValidator
	Tenants        TenantResolver
	Probe          StorageProbe
	AllowedOrigins []string
	OAuthMetadata  *OAuthMetadata
	OAuth          OAuthService
}

// NewRouter monta o roteador com sonda, rotas de autenticação, de notas e de
// administração, servidas sob /v1 e, durante a transição do ADR-0026, também sem
// prefixo e marcadas como obsoletas. As rotas do authorization server OAuth que os
// clientes acessam ficam na raiz, sem versão e fora da lista de origens do CORS, e as
// da tela de consentimento só sob /v1.
func NewRouter(d RouterDeps) chi.Router {
	mustHaveRouterDeps(d)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	mountOAuthServerRoutes(r, d)
	r.Group(func(g chi.Router) {
		g.Use(allowOrigins(d.AllowedOrigins))
		mountAPIRoutes(g, d)
	})
	return r
}

// mountAPIRoutes monta as rotas sob /v1 e o alias obsoleto sem prefixo, que responde
// por todo caminho que nenhuma outra rota atende.
func mountAPIRoutes(r chi.Router, d RouterDeps) {
	routes := newVersionRoutes(d)
	if d.OAuth != nil {
		mountOAuthRequestRoutes(r, d.OAuth, d.Tokens)
		mountOAuthAuthorizationRoutes(r, d.OAuth, d.Tokens)
	}
	r.Mount(currentVersionPrefix, routes)
	r.Mount("/", markDeprecated(routes))
}

// mustHaveRouterDeps barra na montagem as dependências que faltam às rotas ligadas.
func mustHaveRouterDeps(d RouterDeps) {
	if d.Tokens == nil {
		panic("api: Tokens obrigatorio para proteger as rotas")
	}
	if d.Tenants == nil && (d.Notes != nil || d.Admin != nil) {
		panic("api: Tenants obrigatorio para as rotas de notas e de administracao")
	}
	if d.OAuth != nil && d.OAuthMetadata == nil {
		panic("api: OAuthMetadata obrigatorio para as rotas OAuth, que usam o Issuer")
	}
}

func newVersionRoutes(d RouterDeps) chi.Router {
	r := chi.NewRouter()
	r.Get("/healthz", healthz(d.Probe))
	if d.Auth != nil {
		mountAuthRoutes(r, d.Auth, d.Tokens)
	}
	if d.Notes != nil {
		mountProtectedNotesRoutes(r, d.Notes, d.Tokens, d.Tenants)
	}
	if d.Admin != nil {
		mountAdminRoutes(r, d.Admin, d.Tokens, d.Tenants)
	}
	if d.AccountEmail != nil {
		mountAccountEmailRoutes(r, d.AccountEmail, d.Tokens)
	}
	return r
}

func mountProtectedNotesRoutes(r chi.Router, svc NoteService, tokenValidator TokenValidator, tenants TenantResolver) {
	r.Group(func(g chi.Router) {
		g.Use(RequireAuth(tokenValidator), RequireTenant(tenants))
		mountNotesRoutes(g, svc)
	})
}

func mountAuthRoutes(r chi.Router, authSvc AuthService, tokenValidator TokenValidator) {
	h := &authHandler{svc: authSvc}
	r.Post("/auth/login", h.login)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
	r.Post("/auth/invites/accept", h.acceptInvite)
	r.Group(func(g chi.Router) {
		g.Use(RequireAuth(tokenValidator))
		g.Get("/auth/me", h.me)
		g.Get("/me/tenants", h.myTenants)
		admin := g.With(requireOAuthScope(oauth.ScopeAdmin))
		admin.Post("/tenants", h.createTenant)
		admin.Get("/me/invites", h.myInvites)
		admin.Post("/me/invites/{id}/accept", h.acceptMyInvite)
		admin.Post("/me/invites/{id}/decline", h.declineMyInvite)
	})
}

func mountNotesRoutes(r chi.Router, svc NoteService) {
	h := &notesHandler{svc: svc}
	read := r.With(requireOAuthScope(oauth.ScopeNotesRead))
	read.Get("/notes", h.list)
	read.Get("/notes/search", h.search)
	read.Get("/notes/{id}", h.get)
	write := r.With(requireOAuthScope(oauth.ScopeNotesWrite), requireWriter)
	write.Post("/notes", h.create)
	write.Put("/notes/{id}", h.update)
	write.Delete("/notes/{id}", h.delete)
}
