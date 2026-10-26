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

// VaultPublisher publica e despublica o cofre do tenant do ator (ADR-0048).
type VaultPublisher interface {
	Publish(ctx context.Context, actor identity.Actor, vaultID, slug, title string) (publication.Publication, error)
	Unpublish(ctx context.Context, actor identity.Actor, vaultID string) error
	Publication(ctx context.Context, vaultID string) (publication.Publication, error)
}

type publicationHandler struct {
	svc VaultPublisher
}

type publicationResponse struct {
	VaultID     string    `json:"vault_id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	PublishedBy string    `json:"published_by"`
	PublishedAt time.Time `json:"published_at"`
}

// mountPublicationRoutes monta a publicação do cofre, só sob /v1: quem é membro do
// cofre lê, e só owner e admin publicam e despublicam, com o escopo de administração.
func mountPublicationRoutes(r chi.Router, svc VaultPublisher, tokens TokenValidator, tenants TenantResolver) {
	h := &publicationHandler{svc: svc}
	read := r.With(acceptOAuthScope(oauth.ScopeNotesRead), RequireAuth(tokens), RequireTenant(tenants))
	read.Get("/vaults/{id}/publication", h.read)
	admin := r.With(acceptOAuthScope(oauth.ScopeAdmin), RequireAuth(tokens), RequireTenant(tenants), requireAdmin)
	admin.Put("/vaults/{id}/publication", h.publish)
	admin.Delete("/vaults/{id}/publication", h.unpublish)
}

func (h *publicationHandler) read(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Publication(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writePublicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toPublicationResponse(p))
}

func (h *publicationHandler) publish(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	p, err := h.svc.Publish(r.Context(), actor, chi.URLParam(r, "id"), req.Slug, req.Title)
	if err != nil {
		writePublicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toPublicationResponse(p))
}

func (h *publicationHandler) unpublish(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	if err := h.svc.Unpublish(r.Context(), actor, chi.URLParam(r, "id")); err != nil {
		writePublicationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toPublicationResponse(p publication.Publication) publicationResponse {
	return publicationResponse{VaultID: p.VaultID, Slug: p.Slug, Title: p.Title, PublishedBy: p.PublishedBy, PublishedAt: p.PublishedAt}
}

func writePublicationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, publication.ErrInvalidSlug), errors.Is(err, publication.ErrInvalidTitle):
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
	case errors.Is(err, publication.ErrSlugTaken):
		writeError(w, "slug_taken", err.Error(), http.StatusConflict)
	case errors.Is(err, publication.ErrNotPublished):
		writeError(w, "not_published", err.Error(), http.StatusNotFound)
	case errors.Is(err, publication.ErrNotAdmin):
		writeError(w, "forbidden", "acesso restrito a administradores", http.StatusForbidden)
	case errors.Is(err, notes.ErrVaultForbidden):
		writeError(w, "vault_forbidden", notes.ErrVaultForbidden.Error(), http.StatusForbidden)
	default:
		writeLoggedInternal(w, "falha na publicacao", "publicar o cofre", err)
	}
}
