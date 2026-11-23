package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
)

// PublicationService é o que as rotas de publicação consomem (ADR-0048).
type PublicationService interface {
	Preview(ctx context.Context, vaultID string, t publication.Target) (publication.Preview, error)
	Publish(ctx context.Context, actor identity.Actor, vaultID string, req publication.Request) (publication.Publication, error)
	List(ctx context.Context, vaultID string) ([]publication.Publication, error)
	SetIndexable(ctx context.Context, actor identity.Actor, id string, indexable bool) (publication.Publication, error)
	Unpublish(ctx context.Context, actor identity.Actor, id string) error
}

type publicationsHandler struct {
	svc PublicationService
}

// mountPublicationRoutes monta a publicação, que nasceu depois da transição e existe só
// sob /v1, com os escopos das notas. O viewer passa pela rota de escrita e o serviço o
// barra com publish_forbidden.
func mountPublicationRoutes(r chi.Router, svc PublicationService, validator TokenValidator, tenants TenantResolver) {
	h := &publicationsHandler{svc: svc}
	read := r.With(acceptOAuthScope(oauth.ScopeNotesRead), RequireAuth(validator), RequireTenant(tenants))
	write := r.With(acceptOAuthScope(oauth.ScopeNotesWrite), RequireAuth(validator), RequireTenant(tenants))
	read.Get("/vaults/{id}/publications/preview", h.preview)
	write.Post("/vaults/{id}/publications", h.publish)
	read.Get("/publications", h.list)
	write.Patch("/publications/{id}", h.setIndexable)
	write.Delete("/publications/{id}", h.unpublish)
}

type targetRequest struct {
	Kind   string `json:"kind"`
	NoteID string `json:"note_id"`
	Path   string `json:"path"`
}

type publishRequest struct {
	Target    targetRequest `json:"target"`
	Confirm   string        `json:"confirm"`
	Indexable bool          `json:"indexable"`
}

type previewResponse struct {
	Kind           string `json:"kind"`
	TargetName     string `json:"target_name"`
	NoteCount      int    `json:"note_count"`
	IncludesFuture bool   `json:"includes_future"`
}

type publisherResponse struct {
	ID   string  `json:"id"`
	Name *string `json:"name"`
}

// publicationResponse é a publicação; public_id e title_slug só existem na de nota.
type publicationResponse struct {
	ID          string            `json:"id"`
	VaultID     string            `json:"vault_id"`
	VaultName   string            `json:"vault_name"`
	Kind        string            `json:"kind"`
	NoteID      *string           `json:"note_id"`
	FolderPath  *string           `json:"folder_path"`
	TargetName  string            `json:"target_name"`
	NoteCount   int               `json:"note_count"`
	Indexable   bool              `json:"indexable"`
	PublishedBy publisherResponse `json:"published_by"`
	PublishedAt time.Time         `json:"published_at"`
	PublicID    *string           `json:"public_id,omitempty"`
	TitleSlug   *string           `json:"title_slug,omitempty"`
}

type publicationListResponse struct {
	Items []publicationResponse `json:"items"`
	Total int                   `json:"total"`
}

func (h *publicationsHandler) preview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	t := publication.Target{Kind: q.Get("kind"), NoteID: q.Get("note_id"), FolderPath: q.Get("path")}
	p, err := h.svc.Preview(r.Context(), chi.URLParam(r, "id"), t)
	if err != nil {
		writePublicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, previewResponse{Kind: p.Kind, TargetName: p.TargetName, NoteCount: p.NoteCount, IncludesFuture: p.IncludesFuture})
}

func (h *publicationsHandler) publish(w http.ResponseWriter, r *http.Request) {
	var req publishRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	t := publication.Target{Kind: req.Target.Kind, NoteID: req.Target.NoteID, FolderPath: req.Target.Path}
	p, err := h.svc.Publish(r.Context(), actor, chi.URLParam(r, "id"), publication.Request{Target: t, Confirm: req.Confirm, Indexable: req.Indexable})
	writePublicationResult(w, http.StatusCreated, p, err)
}

func (h *publicationsHandler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), r.URL.Query().Get("vault_id"))
	if err != nil {
		writePublicationError(w, err)
		return
	}
	out := publicationListResponse{Items: make([]publicationResponse, 0, len(list)), Total: len(list)}
	for _, p := range list {
		out.Items = append(out.Items, toPublicationResponse(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *publicationsHandler) setIndexable(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Indexable *bool `json:"indexable"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.Indexable == nil {
		writeError(w, "invalid_request", `body has no "indexable", expected {"indexable": true|false}`, http.StatusBadRequest)
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	p, err := h.svc.SetIndexable(r.Context(), actor, chi.URLParam(r, "id"), *req.Indexable)
	writePublicationResult(w, http.StatusOK, p, err)
}

func (h *publicationsHandler) unpublish(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	if err := h.svc.Unpublish(r.Context(), actor, chi.URLParam(r, "id")); err != nil {
		writePublicationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toPublicationResponse(p publication.Publication) publicationResponse {
	out := publicationResponse{
		ID: p.ID, VaultID: p.VaultID, VaultName: p.VaultName, Kind: p.Kind, NoteID: nonEmpty(p.NoteID),
		FolderPath: nonEmpty(p.FolderPath), TargetName: p.TargetName, NoteCount: p.NoteCount, Indexable: p.Indexable,
		PublishedBy: publisherResponse{ID: p.PublishedByID, Name: nonEmpty(p.PublishedByName)}, PublishedAt: p.PublishedAt,
	}
	if p.Kind == publication.KindNote {
		slug := publication.TitleSlug(p.TargetName)
		out.PublicID, out.TitleSlug = nonEmpty(p.PublicID), &slug
	}
	return out
}

// nonEmpty devolve nil para o texto vazio, que vira null no JSON.
func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func writePublicationResult(w http.ResponseWriter, status int, p publication.Publication, err error) {
	if err != nil {
		writePublicationError(w, err)
		return
	}
	writeJSON(w, status, toPublicationResponse(p))
}

// publicationErrors traduz os erros da publicação no status e no código do contrato.
var publicationErrors = []struct {
	err    error
	status int
	code   string
}{
	{publication.ErrInvalidTarget, http.StatusBadRequest, "invalid_request"},
	{publication.ErrConfirmationMismatch, http.StatusUnprocessableEntity, "confirmation_mismatch"},
	{notes.ErrVaultForbidden, http.StatusForbidden, "vault_forbidden"},
	{publication.ErrPublishForbidden, http.StatusForbidden, "publish_forbidden"},
	{publication.ErrNoteNotFound, http.StatusNotFound, "note_not_found"},
	{publication.ErrFolderNotFound, http.StatusNotFound, "folder_not_found"},
	{publication.ErrPublicationNotFound, http.StatusNotFound, "publication_not_found"},
}

func writePublicationError(w http.ResponseWriter, err error) {
	var taken *publication.AlreadyPublishedError
	if errors.As(err, &taken) {
		writeJSON(w, http.StatusConflict, alreadyPublishedResponse{
			errorResponse: errorResponse{Error: "already_published", Message: err.Error()}, PublicationID: taken.PublicationID,
		})
		return
	}
	for _, e := range publicationErrors {
		if errors.Is(err, e.err) {
			writeError(w, e.code, err.Error(), e.status)
			return
		}
	}
	writeLoggedInternal(w, "falha na publicacao", "gerir publicacoes", err)
}

// alreadyPublishedResponse é o 409 com a publicação que já cobre o alvo.
type alreadyPublishedResponse struct {
	errorResponse
	PublicationID string `json:"publication_id"`
}
