package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

// noteRankResponse é a organização derivada da nota, null antes do segundo estágio
// do indexador.
type noteRankResponse struct {
	Degree    *int     `json:"degree"`
	PageRank  *float64 `json:"pagerank"`
	Community *int64   `json:"community"`
}

func toNoteRankResponse(r notes.NoteRank) noteRankResponse {
	return noteRankResponse{Degree: r.Degree, PageRank: r.PageRank, Community: r.Community}
}

// noteV1Response é a nota em /v1: a das rotas sem prefixo mais a organização
// derivada.
type noteV1Response struct {
	noteResponse
	noteRankResponse
}

func toV1Response(n notes.Note) any {
	return noteV1Response{noteResponse: toNoteResponse(n), noteRankResponse: toNoteRankResponse(n.Rank)}
}

// mountV1NotesRoutes monta a nota como item de id global e a coleção das notas sob
// o cofre que as contém (ADR-0040).
func mountV1NotesRoutes(r chi.Router, svc NoteService) {
	h := &notesHandler{svc: svc, render: toV1Response}
	r.Get("/v1/notes/{id}", h.get)
	r.With(requireWriter).Put("/v1/notes/{id}", h.update)
	r.With(requireWriter).Delete("/v1/notes/{id}", h.delete)
	r.Get("/v1/vaults/{id}/notes", h.listVault)
	r.With(requireWriter).Post("/v1/vaults/{id}/notes", h.createInRouteVault)
	r.Get("/v1/search/notes", h.searchV1)
}

// searchHitResponse é o resultado da busca em /v1: a nota resumida, o cofre dela, a
// relevância e o trecho.
type searchHitResponse struct {
	Note    notes.NoteSummary `json:"note"`
	VaultID string            `json:"vault_id"`
	Rank    float64           `json:"rank"`
	Snippet string            `json:"snippet"`
}

func (h *notesHandler) searchV1(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, "invalid_request", "query string 'q' é obrigatória", http.StatusBadRequest)
		return
	}
	offset, limit, ok := parsePage(w, r)
	if !ok {
		return
	}
	results, err := h.svc.SearchNotes(r.Context(), query, r.URL.Query().Get("vault"), offset, limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toSearchHits(results))
}

func toSearchHits(results []notes.SearchResult) []searchHitResponse {
	hits := make([]searchHitResponse, 0, len(results))
	for _, res := range results {
		hits = append(hits, searchHitResponse{Note: notes.Summarize(res.ID, res.Path), VaultID: res.VaultID, Rank: res.Rank, Snippet: res.Snippet})
	}
	return hits
}

func (h *notesHandler) createInRouteVault(w http.ResponseWriter, r *http.Request) {
	vaultID := chi.URLParam(r, "id")
	h.createIn(w, r, func(createNoteRequest) string { return vaultID })
}

func (h *notesHandler) listVault(w http.ResponseWriter, r *http.Request) {
	q, err := parseVaultNotesQuery(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	list, err := h.svc.ListVaultNotes(r.Context(), chi.URLParam(r, "id"), q)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	h.writeNotes(w, list)
}

// parseVaultNotesQuery lê filtro, ordem e página da lista das notas do cofre; a
// direção padrão é decrescente.
func parseVaultNotesQuery(r *http.Request) (notes.VaultNotesQuery, error) {
	qr := queryReader{values: r.URL.Query()}
	q := notes.VaultNotesQuery{
		Orphan:     qr.boolean("orphan"),
		Community:  qr.int64("community"),
		Sort:       qr.oneOf("sort", notes.SortByPageRank, notes.SortByDegree, notes.SortByUpdated),
		Descending: qr.oneOf("direction", "asc", "desc") != "asc",
	}
	q.Offset, q.Limit = qr.page()
	return q, qr.err
}
