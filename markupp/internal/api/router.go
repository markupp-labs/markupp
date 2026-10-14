package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouterWithAuth monta o roteador com sonda, rotas de notas e rotas de autenticação.
func NewRouterWithAuth(
	svc NoteService,
	authSvc AuthService,
	tokenValidator TokenValidator,
	probe StorageProbe,
	allowedOrigins []string,
) chi.Router {
	if tokenValidator == nil {
		panic("api: tokenValidator obrigatorio para proteger rotas de notas")
	}
	r := newBaseRouter(probe, allowedOrigins)
	if authSvc != nil {
		mountAuthRoutes(r, authSvc, tokenValidator)
	}
	mountProtectedNotesRoutes(r, svc, tokenValidator)
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

func mountProtectedNotesRoutes(r chi.Router, svc NoteService, tokenValidator TokenValidator) {
	r.Group(func(g chi.Router) {
		g.Use(RequireAuth(tokenValidator))
		mountNotesRoutes(g, svc)
	})
}

func mountAuthRoutes(r chi.Router, authSvc AuthService, tokenValidator TokenValidator) {
	h := &authHandler{svc: authSvc}
	r.Post("/auth/login", h.login)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
	r.Post("/auth/invites/accept", h.acceptInvite)
	if tokenValidator != nil {
		r.With(RequireAuth(tokenValidator)).Get("/auth/me", h.me)
	}
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
