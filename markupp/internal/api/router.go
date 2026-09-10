package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter monta as rotas REST de notas sobre svc, com RequestID e Recoverer.
func NewRouter(svc NoteService) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	h := &notesHandler{svc: svc}
	r.Post("/notes", h.create)
	r.Get("/notes", h.list)
	r.Get("/notes/search", h.search)
	r.Get("/notes/{id}", h.get)
	r.Put("/notes/{id}", h.update)
	r.Delete("/notes/{id}", h.delete)

	return r
}
