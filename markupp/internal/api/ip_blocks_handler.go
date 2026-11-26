package api

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// IPBlockService é a lista de bloqueios de IP que o administrador da instância mantém.
type IPBlockService interface {
	List(ctx context.Context) ([]instance.IPBlock, error)
	Create(ctx context.Context, req instance.IPBlockRequest) (instance.IPBlock, error)
	Delete(ctx context.Context, actorID, id string) error
}

// BlockedIPs diz se um IP caiu numa faixa bloqueada (ADR-0061).
type BlockedIPs interface {
	Blocked(addr netip.Addr) bool
}

type ipBlockView struct {
	ID        string    `json:"id"`
	CIDR      string    `json:"cidr"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func ipBlockViewOf(b instance.IPBlock) ipBlockView {
	return ipBlockView{ID: b.ID, CIDR: b.Prefix.String(), Note: b.Note, CreatedAt: b.CreatedAt}
}

type ipBlockRequest struct {
	CIDR string `json:"cidr"`
	Note string `json:"note"`
}

type ipBlocksHandler struct {
	svc IPBlockService
}

// mountIPBlockRoutes monta /instance/ip-blocks com o escopo de administração e só para
// quem administra a instância.
func mountIPBlockRoutes(r chi.Router, svc IPBlockService, accounts AuthService, admins instance.Admins, validator TokenValidator) {
	h := &ipBlocksHandler{svc: svc}
	g := r.With(acceptOAuthScope(oauth.ScopeAdmin), RequireAuth(validator), requireInstanceAdmin(accounts, admins), noStore)
	g.Get("/instance/ip-blocks", h.list)
	g.Post("/instance/ip-blocks", h.create)
	g.Delete("/instance/ip-blocks/{id}", h.delete)
}

func (h *ipBlocksHandler) list(w http.ResponseWriter, r *http.Request) {
	blocks, err := h.svc.List(r.Context())
	if err != nil {
		writeIPBlockError(w, err)
		return
	}
	views := make([]ipBlockView, 0, len(blocks))
	for _, b := range blocks {
		views = append(views, ipBlockViewOf(b))
	}
	writeJSON(w, http.StatusOK, views)
}

// create bloqueia a faixa, com o IP de quem pede para ele não se trancar fora.
func (h *ipBlocksHandler) create(w http.ResponseWriter, r *http.Request) {
	var body ipBlockRequest
	if !decodeJSONBody(w, r, &body) {
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	block, err := h.svc.Create(r.Context(), instance.IPBlockRequest{ActorID: actor.UserID,
		ActorIP: auth.OriginFrom(r.Context()).IP, CIDR: body.CIDR, Note: body.Note})
	if err != nil {
		writeIPBlockError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ipBlockViewOf(block))
}

func (h *ipBlocksHandler) delete(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	if err := h.svc.Delete(r.Context(), actor.UserID, chi.URLParam(r, "id")); err != nil {
		writeIPBlockError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeIPBlockError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, instance.ErrInvalidIPBlock):
		writeError(w, "invalid_ip_block", err.Error(), http.StatusBadRequest)
	case errors.Is(err, instance.ErrBlocksOwnIP):
		writeError(w, "blocks_own_ip", err.Error(), http.StatusConflict)
	case errors.Is(err, instance.ErrIPBlockExists):
		writeError(w, "ip_block_exists", "this range is already blocked", http.StatusConflict)
	case errors.Is(err, instance.ErrIPBlockNotFound):
		writeError(w, "ip_block_not_found", "ip block not found", http.StatusNotFound)
	default:
		writeLoggedInternal(w, "falha nos bloqueios de ip", "gerir o bloqueio", err)
	}
}

// refuseBlockedIPs responde 403 ao pedido de IP bloqueado, antes de qualquer rota, menos
// à sonda de saúde, que o orquestrador precisa alcançar. Nil deixa passar tudo.
func refuseBlockedIPs(blocked BlockedIPs) func(http.Handler) http.Handler {
	if blocked == nil {
		return passThrough
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/healthz") && originBlocked(r, blocked) {
				writeError(w, "ip_blocked", "requests from this IP are blocked by the instance administrator", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func originBlocked(r *http.Request, blocked BlockedIPs) bool {
	addr, err := netip.ParseAddr(auth.OriginFrom(r.Context()).IP)
	return err == nil && blocked.Blocked(addr)
}
