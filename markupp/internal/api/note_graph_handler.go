package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

// NoteGraph é a leitura do grafo de ligações que os handlers HTTP consomem.
type NoteGraph interface {
	Backlinks(ctx context.Context, noteID string, offset, limit int) ([]notes.Backlink, error)
	Neighbors(ctx context.Context, noteID string, offset, limit int) ([]notes.Neighbor, error)
	Orphans(ctx context.Context, vaultID string, offset, limit int) ([]notes.LinkedNote, error)
	UnresolvedLinks(ctx context.Context, vaultID string, offset, limit int) ([]notes.UnresolvedLink, error)
	Mentions(ctx context.Context, noteID string, offset, limit int) ([]notes.UnlinkedMention, error)
	Suggestions(ctx context.Context, noteID string, offset, limit int) ([]notes.LinkSuggestion, error)
	Hubs(ctx context.Context, vaultID string, offset, limit int) ([]notes.Hub, error)
	Communities(ctx context.Context, vaultID string, offset, limit int) ([]notes.CommunityMember, error)
}

func mountNoteGraphRoutes(r chi.Router, graph NoteGraph) {
	r.Get("/notes/{id}/backlinks", servePage(graph.Backlinks))
	r.Get("/notes/{id}/neighbors", servePage(graph.Neighbors))
	r.Get("/notes/{id}/mentions", servePage(graph.Mentions))
	r.Get("/notes/{id}/suggestions", servePage(graph.Suggestions))
	r.Get("/vaults/{id}/orphans", servePage(graph.Orphans))
	r.Get("/vaults/{id}/unresolved-links", servePage(graph.UnresolvedLinks))
	r.Get("/vaults/{id}/hubs", servePage(graph.Hubs))
	r.Get("/vaults/{id}/clusters", servePage(graph.Communities))
}

// servePage responde com a página que list devolve para o id da rota, sempre como
// array JSON.
func servePage[T any](list func(ctx context.Context, id string, offset, limit int) ([]T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		offset, limit, ok := parsePage(w, r)
		if !ok {
			return
		}
		page, err := list(r.Context(), chi.URLParam(r, "id"), offset, limit)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if page == nil {
			page = []T{}
		}
		writeJSON(w, http.StatusOK, page)
	}
}
