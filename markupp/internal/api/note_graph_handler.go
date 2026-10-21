package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

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
	Suggestions(ctx context.Context, noteID string, p notes.Page) ([]notes.LinkSuggestion, error)
	Communities(ctx context.Context, vaultID string, p notes.Page) ([]notes.Community, error)
	Community(ctx context.Context, vaultID string, communityID int64) (notes.Community, error)
	VaultGraph(ctx context.Context, vaultID string, f notes.GraphFilter) (notes.Graph, error)
	LocalGraph(ctx context.Context, noteID string, depth int, f notes.GraphFilter) (notes.Graph, error)
	VaultGraphVersion(ctx context.Context, vaultID string) (int64, error)
	NoteGraphVersion(ctx context.Context, noteID string) (int64, error)
}

func mountNoteGraphRoutes(r chi.Router, graph NoteGraph) {
	r.Get("/notes/{id}/links", serveList(readLinkFilter, graph.Links))
	r.Get("/notes/{id}/backlinks", serveList(readLinkFilter, graph.Backlinks))
	r.Get("/vaults/{id}/links", serveList(readVaultLinkQuery, graph.VaultLinks))
	r.Get("/notes/{id}/mentions", serveList(readPage, graph.Mentions))
	r.Get("/notes/{id}/backmentions", serveList(readPage, graph.Backmentions))
	r.Get("/notes/{id}/suggestions", serveList(readPage, graph.Suggestions))
	r.Get("/vaults/{id}/communities", serveList(readPage, graph.Communities))
	r.Get("/vaults/{id}/communities/{cid}", serveCommunity(graph))
	mountGraphViewRoutes(r, graph)
}

// serveList responde com a página que list devolve para o id da rota e a consulta
// que read lê da query string, sempre como array JSON; consulta inválida é 400.
func serveList[Q, T any](read func(*queryReader) Q, list func(ctx context.Context, id string, q Q) ([]T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, ok := readQuery(w, r, read)
		if !ok {
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

// serveCommunity responde com a comunidade do id da rota.
func serveCommunity(graph NoteGraph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := chi.URLParam(r, "cid")
		communityID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, "invalid_id", fmt.Sprintf("id de comunidade %q, esperado inteiro", raw), http.StatusBadRequest)
			return
		}
		community, err := graph.Community(r.Context(), chi.URLParam(r, "id"), communityID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, community)
	}
}
