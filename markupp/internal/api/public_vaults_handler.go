package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
)

// PublicVaultReader é a leitura sem conta dos cofres publicados (ADR-0048). Slug
// inexistente e cofre não publicado dão o mesmo publication.ErrNotPublished.
type PublicVaultReader interface {
	PublicVault(ctx context.Context, slug string) (publication.PublicVault, error)
	PublicNotes(ctx context.Context, slug string, q notes.VaultNotesQuery) ([]notes.Note, error)
	PublicNote(ctx context.Context, slug, noteID string) (notes.Note, error)
	SearchPublicNotes(ctx context.Context, slug, query string, offset, limit int) ([]notes.SearchResult, error)
	PublicGraphVersion(ctx context.Context, slug string) (int64, error)
	PublicGraph(ctx context.Context, slug string, f notes.GraphFilter) (notes.Graph, error)
	PublicLinks(ctx context.Context, slug string, q notes.VaultLinkQuery) ([]notes.Link, error)
}

// publicCacheControl deixa qualquer cache guardar a leitura pública, revalidando pelo
// ETag antes de usar: a edição aparece na leitura seguinte.
const publicCacheControl = "public, no-cache"

// maxPublicQueryLength limita a consulta da busca pública, em bytes.
const maxPublicQueryLength = 256

// publicVaultPath é o prefixo das rotas públicas, só sob /v1.
const publicVaultPath = currentVersionPrefix + "/public/vaults/{slug}"

type publicVaultsHandler struct {
	reader PublicVaultReader
}

// mountPublicVaultRoutes monta a leitura sem conta, fora do CORS por lista de origens:
// qualquer origem lê, sem credenciais, e cada IP tem um limite só para todas as rotas.
// O preflight fica fora do limite.
func mountPublicVaultRoutes(r chi.Router, reader PublicVaultReader, limiter *AccountRateLimiter) {
	if reader == nil {
		return
	}
	h := &publicVaultsHandler{reader: reader}
	r.Route(publicVaultPath, func(p chi.Router) {
		p.Use(allowAnyOrigin, exposePublicHeaders)
		p.Options("/*", openPreflight(http.MethodGet, "If-None-Match"))
		limited := p.With(limitPublicReads(limiter))
		limited.Get("/", h.vault)
		limited.Get("/notes", h.listNotes)
		limited.Get("/notes/{id}", h.readNote)
		limited.Get("/search", h.search)
		limited.Get("/links", h.links)
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

// publicVaultResponse é o cofre publicado: sem tenant, sem id interno e sem quem
// publicou.
type publicVaultResponse struct {
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	PublishedAt  time.Time `json:"published_at"`
	GraphVersion int64     `json:"graph_version"`
}

// publicNoteResponse é a nota lida sem conta: a de /v1 sem o cofre.
type publicNoteResponse struct {
	notes.NoteSummary
	Content       string    `json:"content"`
	ContentSHA256 string    `json:"content_sha256"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	noteRankResponse
}

// publicSearchHitResponse é o resultado da busca pública: o de /v1 sem o cofre.
type publicSearchHitResponse struct {
	Note    notes.NoteSummary `json:"note"`
	Rank    float64           `json:"rank"`
	Snippet string            `json:"snippet"`
}

func (h *publicVaultsHandler) vault(w http.ResponseWriter, r *http.Request) {
	vault, err := h.reader.PublicVault(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeTaggedJSON(w, r, publicVaultResponse{Slug: vault.Slug, Title: vault.Title, PublishedAt: vault.PublishedAt, GraphVersion: vault.GraphVersion})
}

func (h *publicVaultsHandler) listNotes(w http.ResponseWriter, r *http.Request) {
	q, err := parseVaultNotesQuery(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	list, err := h.reader.PublicNotes(r.Context(), chi.URLParam(r, "slug"), q)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]any, 0, len(list))
	for _, n := range list {
		items = append(items, toListItemResponse(n))
	}
	writeTaggedJSON(w, r, items)
}

func (h *publicVaultsHandler) readNote(w http.ResponseWriter, r *http.Request) {
	n, err := h.reader.PublicNote(r.Context(), chi.URLParam(r, "slug"), chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeTaggedJSON(w, r, publicNoteResponse{
		NoteSummary: notes.Summarize(n.ID, n.Path), Content: n.Content, ContentSHA256: n.ContentSHA256,
		CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt, noteRankResponse: toNoteRankResponse(n.Rank),
	})
}

func (h *publicVaultsHandler) search(w http.ResponseWriter, r *http.Request) {
	query, ok := readPublicQuery(w, r)
	if !ok {
		return
	}
	offset, limit, ok := parsePage(w, r)
	if !ok {
		return
	}
	results, err := h.reader.SearchPublicNotes(r.Context(), chi.URLParam(r, "slug"), query, offset, limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	hits := make([]publicSearchHitResponse, 0, len(results))
	for _, res := range results {
		hits = append(hits, publicSearchHitResponse{Note: notes.Summarize(res.ID, res.Path), Rank: res.Rank, Snippet: res.Snippet})
	}
	writeTaggedJSON(w, r, hits)
}

// readPublicQuery lê o q obrigatório da busca, com tamanho limitado.
func readPublicQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, "invalid_request", "query string 'q' é obrigatória", http.StatusBadRequest)
		return "", false
	}
	if len(query) > maxPublicQueryLength {
		writeError(w, "invalid_request", fmt.Sprintf("q com %d bytes, esperado no máximo %d", len(query), maxPublicQueryLength), http.StatusBadRequest)
		return "", false
	}
	return query, true
}

func (h *publicVaultsHandler) links(w http.ResponseWriter, r *http.Request) {
	q, ok := readQuery(w, r, readVaultLinkQuery)
	if !ok {
		return
	}
	links, err := h.reader.PublicLinks(r.Context(), chi.URLParam(r, "slug"), q)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if links == nil {
		links = []notes.Link{}
	}
	writeTaggedJSON(w, r, links)
}

// graph responde como serveGraph, validado pela versão do grafo, mas com cache público.
func (h *publicVaultsHandler) graph(w http.ResponseWriter, r *http.Request) {
	f, ok := readQuery(w, r, readGraphFilter)
	if !ok {
		return
	}
	slug := chi.URLParam(r, "slug")
	version, err := h.reader.PublicGraphVersion(r.Context(), slug)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Cache-Control", publicCacheControl)
	if notModified(w, r, version) {
		return
	}
	graph, err := h.reader.PublicGraph(r.Context(), slug, f)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, graph)
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
