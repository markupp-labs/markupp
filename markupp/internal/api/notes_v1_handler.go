package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

// noteV1Response é a nota em /v1: a das rotas sem prefixo mais a organização
// derivada, null antes do segundo estágio do indexador.
type noteV1Response struct {
	noteResponse
	Degree    *int     `json:"degree"`
	PageRank  *float64 `json:"pagerank"`
	Community *int64   `json:"community"`
}

func toV1Response(n notes.Note) any {
	return noteV1Response{noteResponse: toNoteResponse(n), Degree: n.Rank.Degree, PageRank: n.Rank.PageRank, Community: n.Rank.Community}
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
}

func (h *notesHandler) createInRouteVault(w http.ResponseWriter, r *http.Request) {
	vaultID := chi.URLParam(r, "id")
	h.createIn(w, r, func(createNoteRequest) string { return vaultID })
}

func (h *notesHandler) listVault(w http.ResponseWriter, r *http.Request) {
	q, err := parseVaultNotesQuery(r)
	if err != nil {
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
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
