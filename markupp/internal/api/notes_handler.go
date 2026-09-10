package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

type NoteService interface {
	Create(ctx context.Context, path, content string) (notes.Note, error)
	Update(ctx context.Context, id, path, content string, lastModifiedAt time.Time, force bool) (notes.Note, error)
	Delete(ctx context.Context, id string) error
	GetNoteByID(ctx context.Context, id string) (notes.Note, error)
	ListNotes(ctx context.Context) ([]notes.Note, error)
	SearchNotes(ctx context.Context, query string, offset, limit int) ([]notes.SearchResult, error)
}

type notesHandler struct {
	svc NoteService
}

type noteResponse struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

const (
	maxOffset = 10_000
	maxLimit  = 100
	minLimit  = 1
)

func toResponse(n notes.Note) noteResponse {
	return noteResponse{
		ID:        n.ID,
		Path:      n.Path,
		Content:   n.Content,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}

func (h *notesHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "JSON inválido", http.StatusBadRequest)
		return
	}
	note, err := h.svc.Create(r.Context(), req.Path, req.Content)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toResponse(note))
}

func (h *notesHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Path           string    `json:"path"`
		Content        string    `json:"content"`
		LastModifiedAt time.Time `json:"lastModifiedAt"`
		Force          bool      `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "JSON inválido", http.StatusBadRequest)
		return
	}
	note, err := h.svc.Update(r.Context(), id, req.Path, req.Content, req.LastModifiedAt, req.Force)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(note))
}

func (h *notesHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	note, err := h.svc.GetNoteByID(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(note))
}

func (h *notesHandler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListNotes(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]noteResponse, 0, len(list))
	for _, n := range list {
		out = append(out, toResponse(n))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *notesHandler) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		writeError(w, "invalid_request", "query string 'query' é obrigatória", http.StatusBadRequest)
		return
	}

	offset, err := parseQueryInt(r.URL.Query().Get("offset"), 0)
	if err != nil {
		writeError(w, "invalid_request", "offset deve ser um inteiro", http.StatusBadRequest)
		return
	}

	limit, err := parseQueryInt(r.URL.Query().Get("limit"), 10)
	if err != nil {
		writeError(w, "invalid_request", "limit deve ser um inteiro", http.StatusBadRequest)
		return
	}

	offset = clamp(offset, 0, maxOffset)
	limit = clamp(limit, minLimit, maxLimit)

	results, err := h.svc.SearchNotes(r.Context(), query, offset, limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func parseQueryInt(value string, defaultValue int) (int, error) {
	if value == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(value)
}

func clamp(v, lower, upper int) int {
	if v < lower {
		return lower
	}
	if v > upper {
		return upper
	}
	return v
}

func (h *notesHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code, message string, status int) {
	writeJSON(w, status, errorResponse{Error: code, Message: message})
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, notes.ErrInvalidPath):
		writeError(w, "invalid_path", notes.ErrInvalidPath.Error(), http.StatusBadRequest)
	case errors.Is(err, notes.ErrInvalidContent):
		writeError(w, "invalid_content", notes.ErrInvalidContent.Error(), http.StatusBadRequest)
	case errors.Is(err, notes.ErrDuplicatePath):
		writeError(w, "duplicate_path", notes.ErrDuplicatePath.Error(), http.StatusConflict)
	case errors.Is(err, notes.ErrConflict):
		writeError(w, "conflict", notes.ErrConflict.Error(), http.StatusConflict)
	case errors.Is(err, notes.ErrNotFound):
		writeError(w, "not_found", notes.ErrNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, notes.ErrInvalidID):
		writeError(w, "invalid_id", notes.ErrInvalidID.Error(), http.StatusBadRequest)
	default:
		writeError(w, "internal", "erro interno", http.StatusInternalServerError)
	}
}
