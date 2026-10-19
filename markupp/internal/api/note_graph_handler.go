package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

// NoteGraph é a leitura do grafo de ligações que os handlers HTTP consomem.
type NoteGraph interface {
	Links(ctx context.Context, noteID string, f notes.LinkFilter) ([]notes.Link, error)
	Backlinks(ctx context.Context, noteID string, f notes.LinkFilter) ([]notes.Link, error)
	VaultLinks(ctx context.Context, vaultID string, q notes.VaultLinkQuery) ([]notes.Link, error)
	Mentions(ctx context.Context, noteID string, p notes.Page) ([]notes.Mention, error)
	Backmentions(ctx context.Context, noteID string, p notes.Page) ([]notes.Mention, error)
	Suggestions(ctx context.Context, noteID string, offset, limit int) ([]notes.LinkSuggestion, error)
	Communities(ctx context.Context, vaultID string, offset, limit int) ([]notes.CommunityMember, error)
}

func mountNoteGraphRoutes(r chi.Router, graph NoteGraph) {
	r.Get("/v1/notes/{id}/links", serveList(readLinkFilter, graph.Links))
	r.Get("/v1/notes/{id}/backlinks", serveList(readLinkFilter, graph.Backlinks))
	r.Get("/v1/vaults/{id}/links", serveList(readVaultLinkQuery, graph.VaultLinks))
	r.Get("/v1/notes/{id}/mentions", serveList(readPage, graph.Mentions))
	r.Get("/v1/notes/{id}/backmentions", serveList(readPage, graph.Backmentions))
	r.Get("/notes/{id}/suggestions", servePage(graph.Suggestions))
	r.Get("/vaults/{id}/clusters", servePage(graph.Communities))
}

// serveList responde com a página que list devolve para o id da rota e a consulta
// que read lê da query string, sempre como array JSON; consulta inválida é 400.
func serveList[Q, T any](read func(*queryReader) Q, list func(ctx context.Context, id string, q Q) ([]T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		qr := queryReader{values: r.URL.Query()}
		q := read(&qr)
		if qr.err != nil {
			writeDomainError(w, qr.err)
			return
		}
		page, err := list(r.Context(), chi.URLParam(r, "id"), q)
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

func readPage(qr *queryReader) notes.Page {
	var p notes.Page
	p.Offset, p.Limit = qr.page()
	return p
}

func readLinkFilter(qr *queryReader) notes.LinkFilter {
	return notes.LinkFilter{Type: qr.oneOf("type", notes.LinkExplicit, notes.LinkEmbed, notes.LinkTyped, notes.LinkInferred), Page: readPage(qr)}
}

func readVaultLinkQuery(qr *queryReader) notes.VaultLinkQuery {
	return notes.VaultLinkQuery{Status: qr.oneOf("status", notes.LinkResolved, notes.LinkUnresolved), LinkFilter: readLinkFilter(qr)}
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
