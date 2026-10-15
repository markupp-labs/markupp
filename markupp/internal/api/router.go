package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// RouterDeps reúne o que o roteador expõe. As rotas de um serviço nil não são montadas.
type RouterDeps struct {
	Notes          NoteService
	Auth           AuthService
	Admin          AdminService
	Tokens         TokenValidator
	Tenants        TenantResolver
	Probe          StorageProbe
	AllowedOrigins []string
}

// NewRouter monta o roteador com sonda, rotas de autenticação, de notas e de
// administração.
func NewRouter(d RouterDeps) chi.Router {
	if d.Tokens == nil {
		panic("api: Tokens obrigatorio para proteger as rotas")
	}
	if d.Tenants == nil && (d.Notes != nil || d.Admin != nil) {
		panic("api: Tenants obrigatorio para as rotas de notas e de administracao")
	}
	r := newBaseRouter(d.Probe, d.AllowedOrigins)
	if d.Auth != nil {
		mountAuthRoutes(r, d.Auth, d.Tokens)
	}
	if d.Notes != nil {
		mountProtectedNotesRoutes(r, d.Notes, d.Tokens, d.Tenants)
	}
	if d.Admin != nil {
		mountAdminRoutes(r, d.Admin, d.Tokens, d.Tenants)
	}
	return r
}

func newBaseRouter(probe StorageProbe, allowedOrigins []string) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(allowOrigins(allowedOrigins))
	r.Get("/healthz", healthz(probe))
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
	r.With(RequireAuth(tokenValidator)).Get("/auth/me", h.me)
	r.With(RequireAuth(tokenValidator)).Get("/me/tenants", h.myTenants)
}

func mountNotesRoutes(r chi.Router, svc NoteService) {
	h := &notesHandler{svc: svc}
	r.Post("/notes", h.create)
	r.Get("/notes", h.list)
	r.Get("/notes/search", h.search)
	r.Get("/notes/{id}", h.get)
	r.Put("/notes/{id}", h.update)
	r.Delete("/notes/{id}", h.delete)
}
