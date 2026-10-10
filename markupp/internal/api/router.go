package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter monta as rotas do servidor sobre svc: RequestID, Recoverer, CORS
// para allowedOrigins, a sonda de saúde sobre probe e a API REST de notas.
func NewRouter(svc NoteService, probe StorageProbe, allowedOrigins []string) chi.Router {
	return NewRouterWithUsers(svc, nil, probe, allowedOrigins)
}

// NewRouterWithUsers monta o roteador com suporte a multi-tenant e gestao de usuarios.
func NewRouterWithUsers(notesSvc NoteService, usersSvc UsersService, probe StorageProbe, allowedOrigins []string) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(allowOrigins(allowedOrigins))
	r.Use(TenantMiddleware("default"))

	r.Get("/healthz", healthz(probe))

	notesH := &notesHandler{svc: notesSvc}
	r.Post("/notes", notesH.create)
	r.Get("/notes", notesH.list)
	r.Get("/notes/search", notesH.search)
	r.Get("/notes/{id}", notesH.get)
	r.Put("/notes/{id}", notesH.update)
	r.Delete("/notes/{id}", notesH.delete)

	if usersSvc != nil {
		usersH := &usersHandler{svc: usersSvc}
		r.Post("/users/bootstrap-admin", usersH.bootstrapAdmin)
		r.With(RequireAdmin).Post("/users/invite", usersH.invite)
		r.With(RequireAdmin).Get("/users", usersH.list)
		r.With(RequireAdmin).Get("/users/invites", usersH.listInvites)
	}

	return r
}
