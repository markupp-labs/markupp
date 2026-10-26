// Package api expõe as notas como uma API REST, traduzindo os erros de
// domínio em códigos de status HTTP.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
)

// NoteService é a fatia do serviço de notas que os handlers HTTP consomem.
type NoteService interface {
	Create(ctx context.Context, vaultID, path, content string) (notes.Note, error)
	Update(ctx context.Context, id, path, content string, lastModifiedAt time.Time, force bool) (notes.Note, error)
	Delete(ctx context.Context, id string) error
	GetNoteByID(ctx context.Context, id string) (notes.Note, error)
	ListNotes(ctx context.Context) ([]notes.Note, error)
	ListVaultNotes(ctx context.Context, vaultID string, q notes.VaultNotesQuery) ([]notes.Note, error)
	SearchNotes(ctx context.Context, query, vaultID string, offset, limit int) ([]notes.SearchResult, error)
}

// notesHandler serve as notas pelo serviço, e render dá a representação de cada
// nota na resposta, que muda entre as rotas sem prefixo e as de /v1.
type notesHandler struct {
	svc    NoteService
	render func(notes.Note) any
}

// createNoteRequest é o corpo da criação; as rotas de /v1 ignoram VaultID e tiram
// o cofre do caminho.
type createNoteRequest struct {
	VaultID string `json:"vault_id"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

type noteResponse struct {
	ID            string    `json:"id"`
	VaultID       string    `json:"vault_id"`
	Path          string    `json:"path"`
	Content       string    `json:"content"`
	ContentSHA256 string    `json:"content_sha256"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
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

func toResponse(n notes.Note) any {
	return toNoteResponse(n)
}

func toNoteResponse(n notes.Note) noteResponse {
	return noteResponse{
		ID:            n.ID,
		VaultID:       n.VaultID,
		Path:          n.Path,
		Content:       n.Content,
		ContentSHA256: n.ContentSHA256,
		CreatedAt:     n.CreatedAt,
		UpdatedAt:     n.UpdatedAt,
	}
}

func (h *notesHandler) create(w http.ResponseWriter, r *http.Request) {
	h.createIn(w, r, func(req createNoteRequest) string { return req.VaultID })
}

// createIn cria a nota do corpo no cofre que vaultOf escolhe.
func (h *notesHandler) createIn(w http.ResponseWriter, r *http.Request, vaultOf func(createNoteRequest) string) {
	var req createNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "JSON inválido", http.StatusBadRequest)
		return
	}
	note, err := h.svc.Create(r.Context(), vaultOf(req), req.Path, req.Content)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, h.render(note))
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
	writeJSON(w, http.StatusOK, h.render(note))
}

func (h *notesHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	note, err := h.svc.GetNoteByID(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.render(note))
}

func (h *notesHandler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListNotes(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeNotes(w, list, h.render)
}

// writeNotes responde com as notas na representação render, sempre como array.
func writeNotes(w http.ResponseWriter, list []notes.Note, render func(notes.Note) any) {
	out := make([]any, 0, len(list))
	for _, n := range list {
		out = append(out, render(n))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *notesHandler) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		writeError(w, "invalid_request", "query string 'query' é obrigatória", http.StatusBadRequest)
		return
	}
	offset, limit, ok := parsePage(w, r)
	if !ok {
		return
	}

	results, err := h.svc.SearchNotes(r.Context(), query, "", offset, limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// parsePage lê offset e limit da query string, limitados ao intervalo aceito, ou
// responde 400 e devolve ok falso quando um deles não é inteiro.
func parsePage(w http.ResponseWriter, r *http.Request) (offset, limit int, ok bool) {
	offset, limit, err := pageParams(r.URL.Query())
	if err != nil {
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
		return 0, 0, false
	}
	return offset, limit, true
}

// pageParams lê offset e limit, limitados ao intervalo aceito.
func pageParams(values url.Values) (offset, limit int, err error) {
	offset, err = parseQueryInt(values.Get("offset"), 0)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: offset deve ser um inteiro, recebido %q", errInvalidQuery, values.Get("offset"))
	}
	limit, err = parseQueryInt(values.Get("limit"), 10)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: limit deve ser um inteiro, recebido %q", errInvalidQuery, values.Get("limit"))
	}
	return clamp(offset, 0, maxOffset), clamp(limit, minLimit, maxLimit), nil
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
	case errors.Is(err, errInvalidQuery):
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
	case errors.Is(err, notes.ErrInvalidPath):
		writeError(w, "invalid_path", err.Error(), http.StatusBadRequest)
	case errors.Is(err, notes.ErrInvalidContent):
		writeError(w, "invalid_content", notes.ErrInvalidContent.Error(), http.StatusBadRequest)
	case errors.Is(err, notes.ErrDuplicatePath):
		writeError(w, "duplicate_path", notes.ErrDuplicatePath.Error(), http.StatusConflict)
	case errors.Is(err, notes.ErrVaultForbidden):
		writeError(w, "vault_forbidden", notes.ErrVaultForbidden.Error(), http.StatusForbidden)
	case errors.Is(err, notes.ErrConflict):
		writeError(w, "conflict", notes.ErrConflict.Error(), http.StatusConflict)
	case errors.Is(err, notes.ErrNotFound):
		writeError(w, "not_found", notes.ErrNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, publication.ErrNotPublished):
		writeError(w, "not_found", "cofre publico nao encontrado", http.StatusNotFound)
	case errors.Is(err, notes.ErrCommunityNotFound):
		writeError(w, "not_found", notes.ErrCommunityNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, notes.ErrInvalidID):
		writeError(w, "invalid_id", notes.ErrInvalidID.Error(), http.StatusBadRequest)
	default:
		writeError(w, "internal", "erro interno", http.StatusInternalServerError)
	}
}
