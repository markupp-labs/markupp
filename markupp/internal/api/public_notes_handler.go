package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
)

// PublicNoteReader é a leitura sem conta das notas públicas (ADR-0048). Id inexistente,
// fora do formato e de nota não publicada dão o mesmo publication.ErrPublicNoteNotFound.
type PublicNoteReader interface {
	PublicNote(ctx context.Context, publicID string) (publication.PublicNote, error)
	PublicTree(ctx context.Context, publicID string, offset, limit int) ([]publication.PublicTreeItem, int, error)
	SearchPublic(ctx context.Context, publicID, query string, offset, limit int) ([]publication.PublicSearchHit, int, error)
	PublicGraphTag(ctx context.Context, publicID string) (string, error)
	PublicGraph(ctx context.Context, publicID string) (publication.PublicGraph, error)
}

// publicCacheControl deixa qualquer cache guardar a leitura pública, revalidando pelo
// ETag antes de usar: a edição e a despublicação aparecem na leitura seguinte.
const publicCacheControl = "public, no-cache"

// maxPublicQueryLength limita a consulta da busca pública, em bytes.
const maxPublicQueryLength = 256

// publicNotePath é o prefixo das rotas públicas, só sob /v1.
const publicNotePath = currentVersionPrefix + "/public/notes/{public_id}"

type publicNotesHandler struct {
	reader PublicNoteReader
}

// mountPublicNoteRoutes monta a leitura sem conta, fora do CORS por lista de origens:
// qualquer origem lê, sem credenciais, e o limite vale para todas as rotas juntas. O
// preflight fica fora do limite.
func mountPublicNoteRoutes(r chi.Router, reader PublicNoteReader, limit func(http.Handler) http.Handler) {
	if reader == nil {
		return
	}
	h := &publicNotesHandler{reader: reader}
	r.Route(publicNotePath, func(p chi.Router) {
		p.Use(allowAnyOrigin, exposePublicHeaders)
		p.Options("/*", openPreflight(http.MethodGet, "If-None-Match"))
		p.Options("/", openPreflight(http.MethodGet, "If-None-Match"))
		limited := p.With(limit)
		limited.Get("/", h.note)
		limited.Get("/tree", h.tree)
		limited.Get("/search", h.search)
		limited.With(middleware.Compress(graphCompressionLevel)).Get("/graph", h.graph)
	})
}

// exposePublicHeaders deixa o script de outra origem ler o ETag, para revalidar, e o
// Retry-After do 429.
func exposePublicHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Expose-Headers", "ETag, Retry-After")
		next.ServeHTTP(w, r)
	})
}

type publicScopeResponse struct {
	Kind         string  `json:"kind"`
	Title        string  `json:"title"`
	RootPublicID *string `json:"root_public_id"`
}

type publicLinkResponse struct {
	CitedTitle string  `json:"cited_title"`
	Kind       string  `json:"kind"`
	PublicID   *string `json:"public_id"`
	TitleSlug  *string `json:"title_slug"`
}

// publicNoteResponse é a nota lida sem conta: sem id interno, cofre nem tenant.
type publicNoteResponse struct {
	PublicID    string               `json:"public_id"`
	Title       string               `json:"title"`
	TitleSlug   string               `json:"title_slug"`
	Content     string               `json:"content"`
	UpdatedAt   time.Time            `json:"updated_at"`
	Indexable   bool                 `json:"indexable"`
	Publication publicScopeResponse  `json:"publication"`
	Links       []publicLinkResponse `json:"links"`
}

type publicTreeItemResponse struct {
	PublicID  string `json:"public_id"`
	Title     string `json:"title"`
	TitleSlug string `json:"title_slug"`
	Path      string `json:"path"`
}

type publicSearchHitResponse struct {
	PublicID  string  `json:"public_id"`
	Title     string  `json:"title"`
	TitleSlug string  `json:"title_slug"`
	Snippet   string  `json:"snippet"`
	Rank      float64 `json:"rank"`
}

type publicGraphNodeResponse struct {
	PublicID  string `json:"public_id"`
	Title     string `json:"title"`
	TitleSlug string `json:"title_slug"`
	Degree    *int   `json:"degree"`
}

type publicGraphLinkResponse struct {
	Source int    `json:"source"`
	Target int    `json:"target"`
	Type   string `json:"type"`
}

type publicGraphResponse struct {
	Nodes []publicGraphNodeResponse `json:"nodes"`
	Links []publicGraphLinkResponse `json:"links"`
}

// publicPageResponse é uma página da publicação com o total de itens.
type publicPageResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

func (h *publicNotesHandler) note(w http.ResponseWriter, r *http.Request) {
	n, err := h.reader.PublicNote(r.Context(), chi.URLParam(r, "public_id"))
	if err != nil {
		writePublicError(w, err)
		return
	}
	writeTaggedJSON(w, r, toPublicNoteResponse(n))
}

func toPublicNoteResponse(n publication.PublicNote) publicNoteResponse {
	links := make([]publicLinkResponse, 0, len(n.Links))
	for _, l := range n.Links {
		links = append(links, publicLinkResponse{CitedTitle: l.CitedTitle, Kind: l.Kind, PublicID: l.PublicID, TitleSlug: slugOf(l.TargetTitle)})
	}
	return publicNoteResponse{
		PublicID: n.PublicID, Title: n.Title, TitleSlug: publication.TitleSlug(n.Title), Content: n.Content,
		UpdatedAt: n.UpdatedAt, Indexable: n.Indexable, Links: links,
		Publication: publicScopeResponse{Kind: n.Publication.Kind, Title: n.Publication.Title, RootPublicID: n.Publication.RootPublicID},
	}
}

// slugOf é o title_slug do título que pode faltar.
func slugOf(title *string) *string {
	if title == nil {
		return nil
	}
	slug := publication.TitleSlug(*title)
	return &slug
}

func (h *publicNotesHandler) tree(w http.ResponseWriter, r *http.Request) {
	offset, limit, ok := parsePage(w, r)
	if !ok {
		return
	}
	items, total, err := h.reader.PublicTree(r.Context(), chi.URLParam(r, "public_id"), offset, limit)
	if err != nil {
		writePublicError(w, err)
		return
	}
	out := publicPageResponse[publicTreeItemResponse]{Items: make([]publicTreeItemResponse, 0, len(items)), Total: total}
	for _, i := range items {
		out.Items = append(out.Items, publicTreeItemResponse{PublicID: i.PublicID, Title: i.Title, TitleSlug: publication.TitleSlug(i.Title), Path: i.Path})
	}
	writeTaggedJSON(w, r, out)
}

func (h *publicNotesHandler) search(w http.ResponseWriter, r *http.Request) {
	query, ok := readPublicQuery(w, r)
	if !ok {
		return
	}
	offset, limit, ok := parsePage(w, r)
	if !ok {
		return
	}
	hits, total, err := h.reader.SearchPublic(r.Context(), chi.URLParam(r, "public_id"), query, offset, limit)
	if err != nil {
		writePublicError(w, err)
		return
	}
	out := publicPageResponse[publicSearchHitResponse]{Items: make([]publicSearchHitResponse, 0, len(hits)), Total: total}
	for _, hit := range hits {
		out.Items = append(out.Items, publicSearchHitResponse{PublicID: hit.PublicID, Title: hit.Title, TitleSlug: publication.TitleSlug(hit.Title), Snippet: hit.Snippet, Rank: hit.Rank})
	}
	writeTaggedJSON(w, r, out)
}

// readPublicQuery lê o q obrigatório da busca, com tamanho limitado.
func readPublicQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, "invalid_request", "query string parameter 'q' is required", http.StatusBadRequest)
		return "", false
	}
	if len(query) > maxPublicQueryLength {
		writeError(w, "invalid_request", fmt.Sprintf("q has %d bytes, expected at most %d", len(query), maxPublicQueryLength), http.StatusBadRequest)
		return "", false
	}
	return query, true
}

// graph responde validado pela versão do grafo do cofre e pela publicação, com cache público.
func (h *publicNotesHandler) graph(w http.ResponseWriter, r *http.Request) {
	publicID := chi.URLParam(r, "public_id")
	tag, err := h.reader.PublicGraphTag(r.Context(), publicID)
	if err != nil {
		writePublicError(w, err)
		return
	}
	w.Header().Set("Cache-Control", publicCacheControl)
	if notModifiedSince(w, r, tag) {
		return
	}
	g, err := h.reader.PublicGraph(r.Context(), publicID)
	if err != nil {
		writePublicError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toPublicGraphResponse(g))
}

func toPublicGraphResponse(g publication.PublicGraph) publicGraphResponse {
	out := publicGraphResponse{Nodes: make([]publicGraphNodeResponse, 0, len(g.Nodes)), Links: make([]publicGraphLinkResponse, 0, len(g.Links))}
	for _, n := range g.Nodes {
		out.Nodes = append(out.Nodes, publicGraphNodeResponse{PublicID: n.PublicID, Title: n.Title, TitleSlug: publication.TitleSlug(n.Title), Degree: n.Degree})
	}
	for _, l := range g.Links {
		out.Links = append(out.Links, publicGraphLinkResponse(l))
	}
	return out
}

// writePublicError responde 404 igual para toda nota que não é pública, sem ecoar o id.
func writePublicError(w http.ResponseWriter, err error) {
	if errors.Is(err, publication.ErrPublicNoteNotFound) {
		writeError(w, "not_found", "note not found", http.StatusNotFound)
		return
	}
	writeLoggedInternal(w, "falha na leitura publica", "ler nota publica", err)
}

// writeTaggedJSON responde body com o ETag do hash do JSON e cache público, ou 304 sem
// corpo quando o If-None-Match já traz esse ETag.
func writeTaggedJSON(w http.ResponseWriter, r *http.Request, body any) {
	encoded, err := json.Marshal(body)
	if err != nil {
		writeLoggedInternal(w, "falha na leitura publica", "serializar a resposta", err)
		return
	}
	sum := sha256.Sum256(encoded)
	tag := `"` + hex.EncodeToString(sum[:16]) + `"`
	w.Header().Set("ETag", tag)
	w.Header().Set("Cache-Control", publicCacheControl)
	if matchesETag(r.Header.Get("If-None-Match"), tag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}
