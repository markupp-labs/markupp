package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/linkrefusals"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// LinkRefusalService é o que as rotas da recusa de ligação consomem.
type LinkRefusalService interface {
	Refuse(ctx context.Context, actor identity.Actor, vaultID, noteA, noteB string) (linkrefusals.Refusal, error)
	List(ctx context.Context, vaultID string, p notes.Page) ([]linkrefusals.Refusal, error)
	Undo(ctx context.Context, actor identity.Actor, vaultID, id string) error
}

type linkRefusalsHandler struct {
	svc LinkRefusalService
}

// linkRefusalRequest é o par recusado, sem sentido.
type linkRefusalRequest struct {
	NoteIDs []string `json:"note_ids"`
}

// mountLinkRefusalRoutes monta a recusa de ligação, que existe só sob /v1, com os
// escopos das notas; o serviço barra o viewer.
func mountLinkRefusalRoutes(r chi.Router, svc LinkRefusalService, validator TokenValidator, tenants TenantResolver) {
	h := &linkRefusalsHandler{svc: svc}
	read := r.With(acceptOAuthScope(oauth.ScopeNotesRead), RequireAuth(validator), RequireTenant(tenants))
	write := r.With(acceptOAuthScope(oauth.ScopeNotesWrite), RequireAuth(validator), RequireTenant(tenants))
	read.Get("/vaults/{id}/link-refusals", h.list)
	write.Post("/vaults/{id}/link-refusals", h.create)
	write.Delete("/vaults/{id}/link-refusals/{refusal_id}", h.undo)
}

func (h *linkRefusalsHandler) list(w http.ResponseWriter, r *http.Request) {
	page, ok := readQuery(w, r, readPage)
	if !ok {
		return
	}
	list, err := h.svc.List(r.Context(), chi.URLParam(r, "id"), page)
	if err != nil {
		writeLinkRefusalError(w, err)
		return
	}
	if list == nil {
		list = []linkrefusals.Refusal{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *linkRefusalsHandler) create(w http.ResponseWriter, r *http.Request) {
	var req linkRefusalRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if len(req.NoteIDs) != 2 {
		writeError(w, "invalid_request", fmt.Sprintf("note_ids %q: expected exactly two note ids", req.NoteIDs), http.StatusBadRequest)
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	refusal, err := h.svc.Refuse(r.Context(), actor, chi.URLParam(r, "id"), req.NoteIDs[0], req.NoteIDs[1])
	if err != nil {
		writeLinkRefusalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, refusal)
}

func (h *linkRefusalsHandler) undo(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	if err := h.svc.Undo(r.Context(), actor, chi.URLParam(r, "id"), chi.URLParam(r, "refusal_id")); err != nil {
		writeLinkRefusalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// linkRefusalErrors traduz os erros da recusa de ligação no status e no código do contrato.
var linkRefusalErrors = []struct {
	err    error
	status int
	code   string
}{
	{linkrefusals.ErrReadOnly, http.StatusForbidden, "read_only"},
	{linkrefusals.ErrSameNote, http.StatusBadRequest, "same_note"},
	{linkrefusals.ErrNoteOutsideVault, http.StatusUnprocessableEntity, "note_outside_vault"},
	{linkrefusals.ErrRefusalNotFound, http.StatusNotFound, "link_refusal_not_found"},
	{notes.ErrVaultForbidden, http.StatusForbidden, "vault_forbidden"},
}

func writeLinkRefusalError(w http.ResponseWriter, err error) {
	for _, e := range linkRefusalErrors {
		if errors.Is(err, e.err) {
			writeError(w, e.code, err.Error(), e.status)
			return
		}
	}
	writeLoggedInternal(w, "falha na recusa de ligacao", "recusar ligacao", err)
}
