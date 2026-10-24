package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/vaults"
)

// VaultService é o que as rotas de gestão de cofres consomem.
type VaultService interface {
	List(ctx context.Context) ([]vaults.Vault, error)
	Create(ctx context.Context, actor identity.Actor, name string) (vaults.Vault, error)
	Rename(ctx context.Context, actor identity.Actor, id, name string) (vaults.Vault, error)
	Delete(ctx context.Context, actor identity.Actor, id string) error
	ListMembers(ctx context.Context, vaultID string) ([]vaults.Member, error)
	AddMember(ctx context.Context, actor identity.Actor, vaultID, userID string) (vaults.Member, error)
	RemoveMember(ctx context.Context, actor identity.Actor, vaultID, userID string) error
}

type vaultsHandler struct {
	svc VaultService
}

type vaultResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type vaultNameRequest struct {
	Name string `json:"name"`
}

// mountVaultRoutes monta a gestão de cofres, que nasceu depois da transição e existe só
// sob /v1, com os escopos das notas. O viewer passa pela rota de escrita porque sai
// sozinho do cofre; o serviço barra o resto.
func mountVaultRoutes(r chi.Router, svc VaultService, validator TokenValidator, tenants TenantResolver) {
	h := &vaultsHandler{svc: svc}
	read := r.With(acceptOAuthScope(oauth.ScopeNotesRead), RequireAuth(validator), RequireTenant(tenants))
	write := r.With(acceptOAuthScope(oauth.ScopeNotesWrite), RequireAuth(validator), RequireTenant(tenants))
	read.Get("/vaults", h.list)
	write.Post("/vaults", h.create)
	write.Patch("/vaults/{id}", h.rename)
	write.Delete("/vaults/{id}", h.delete)
	read.Get("/vaults/{id}/members", h.listMembers)
	write.Post("/vaults/{id}/members", h.addMember)
	write.Delete("/vaults/{id}/members/{user_id}", h.removeMember)
}

func (h *vaultsHandler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context())
	if err != nil {
		writeVaultError(w, err)
		return
	}
	out := make([]vaultResponse, 0, len(list))
	for _, v := range list {
		out = append(out, toVaultResponse(v))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *vaultsHandler) create(w http.ResponseWriter, r *http.Request) {
	var req vaultNameRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	v, err := h.svc.Create(r.Context(), actor, req.Name)
	writeVaultResult(w, http.StatusCreated, v, err)
}

func (h *vaultsHandler) rename(w http.ResponseWriter, r *http.Request) {
	var req vaultNameRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	v, err := h.svc.Rename(r.Context(), actor, chi.URLParam(r, "id"), req.Name)
	writeVaultResult(w, http.StatusOK, v, err)
}

func (h *vaultsHandler) delete(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	writeVaultNoContent(w, h.svc.Delete(r.Context(), actor, chi.URLParam(r, "id")))
}

func (h *vaultsHandler) listMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.svc.ListMembers(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeVaultError(w, err)
		return
	}
	out := make([]memberResponse, 0, len(members))
	for _, m := range members {
		out = append(out, toVaultMemberResponse(m))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *vaultsHandler) addMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	m, err := h.svc.AddMember(r.Context(), actor, chi.URLParam(r, "id"), req.UserID)
	if err != nil {
		writeVaultError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toVaultMemberResponse(m))
}

func (h *vaultsHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	writeVaultNoContent(w, h.svc.RemoveMember(r.Context(), actor, chi.URLParam(r, "id"), chi.URLParam(r, "user_id")))
}

func toVaultResponse(v vaults.Vault) vaultResponse {
	return vaultResponse{ID: v.ID, Name: v.Name, CreatedAt: v.CreatedAt}
}

func toVaultMemberResponse(m vaults.Member) memberResponse {
	return memberResponse{ID: m.UserID, Email: m.Email, Role: m.Role, JoinedAt: m.JoinedAt}
}

func writeVaultResult(w http.ResponseWriter, status int, v vaults.Vault, err error) {
	if err != nil {
		writeVaultError(w, err)
		return
	}
	writeJSON(w, status, toVaultResponse(v))
}

func writeVaultNoContent(w http.ResponseWriter, err error) {
	if err != nil {
		writeVaultError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// vaultErrors traduz os erros da gestão de cofres no status e no código do contrato.
var vaultErrors = []struct {
	err    error
	status int
	code   string
}{
	{vaults.ErrInvalidName, http.StatusBadRequest, "invalid_name"},
	{vaults.ErrReadOnly, http.StatusForbidden, "read_only"},
	{vaults.ErrVaultNotFound, http.StatusForbidden, "vault_forbidden"},
	{vaults.ErrNotTenantMember, http.StatusUnprocessableEntity, "not_tenant_member"},
	{vaults.ErrAlreadyVaultMember, http.StatusConflict, "already_vault_member"},
	{vaults.ErrMemberNotFound, http.StatusNotFound, "vault_member_not_found"},
}

func writeVaultError(w http.ResponseWriter, err error) {
	for _, e := range vaultErrors {
		if errors.Is(err, e.err) {
			writeError(w, e.code, err.Error(), e.status)
			return
		}
	}
	writeLoggedInternal(w, "falha na gestao de cofres", "gerir cofres", err)
}
