package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter monta as rotas do servidor sobre svc: RequestID, Recoverer, CORS
// para allowedOrigins, a sonda de saúde sobre probe e a API REST de notas.
func NewRouter(svc NoteService, probe StorageProbe, allowedOrigins []string) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(allowOrigins(allowedOrigins))

	r.Get("/healthz", healthz(probe))

	h := &notesHandler{svc: svc}
	r.Post("/notes", h.create)
	r.Get("/notes", h.list)
	r.Get("/notes/search", h.search)
	r.Get("/notes/{id}", h.get)
	r.Put("/notes/{id}", h.update)
	r.Delete("/notes/{id}", h.delete)

	return r
}
