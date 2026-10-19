package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

type oauthRequestResponse struct {
	ID                string   `json:"id"`
	ClientID          string   `json:"client_id"`
	ClientName        string   `json:"client_name"`
	ClientSource      string   `json:"client_source"`
	RedirectHost      string   `json:"redirect_host"`
	Scopes            []string `json:"scopes"`
	AlreadyAuthorized bool     `json:"already_authorized"`
	ExpiresAt         string   `json:"expires_at"`
}

type oauthDecisionResponse struct {
	RedirectTo string `json:"redirect_to"`
}

// requestDecision é Approve ou Deny do serviço, que devolvem o redirect ao cliente.
type requestDecision func(ctx context.Context, requestID, userID string) (string, error)

// mountOAuthRequestRoutes monta as rotas da tela de consentimento do painel só sob /v1,
// sem o alias obsoleto sem prefixo: exigem login, mas não tenant. O id do pedido funciona
// como portador (ADR-0040): a conta logada que o apresenta pode ver e decidir o pedido.
func mountOAuthRequestRoutes(r chi.Router, svc OAuthService, tokenValidator TokenValidator) {
	h := &oauthHandler{svc: svc}
	r.Route(currentVersionPrefix+"/oauth/requests", func(g chi.Router) {
		g.Use(RequireAuth(tokenValidator))
		g.Get("/{id}", h.describeRequest)
		g.Post("/{id}/approve", h.approveRequest)
		g.Post("/{id}/deny", h.denyRequest)
	})
}

func (h *oauthHandler) describeRequest(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	view, err := h.svc.DescribeRequest(r.Context(), chi.URLParam(r, "id"), actor.UserID)
	if err != nil {
		writeOAuthRequestError(w, "descrever pedido oauth", err)
		return
	}
	writeJSON(w, http.StatusOK, newOAuthRequestResponse(view))
}

func newOAuthRequestResponse(v oauth.RequestView) oauthRequestResponse {
	scopes := v.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	return oauthRequestResponse{
		ID: v.ID, ClientID: v.ClientID, ClientName: v.ClientName, ClientSource: v.ClientSource, RedirectHost: v.RedirectHost,
		Scopes: scopes, AlreadyAuthorized: v.AlreadyAuthorized, ExpiresAt: v.ExpiresAt,
	}
}

func (h *oauthHandler) approveRequest(w http.ResponseWriter, r *http.Request) {
	decideRequest(w, r, h.svc.Approve, "aprovar pedido oauth")
}

func (h *oauthHandler) denyRequest(w http.ResponseWriter, r *http.Request) {
	decideRequest(w, r, h.svc.Deny, "recusar pedido oauth")
}

// decideRequest responde sem cache, porque o redirect da aprovação carrega o código.
func decideRequest(w http.ResponseWriter, r *http.Request, decide requestDecision, operation string) {
	w.Header().Set("Cache-Control", "no-store")
	actor, _ := identity.ActorFrom(r.Context())
	redirectTo, err := decide(r.Context(), chi.URLParam(r, "id"), actor.UserID)
	if err != nil {
		writeOAuthRequestError(w, operation, err)
		return
	}
	writeJSON(w, http.StatusOK, oauthDecisionResponse{RedirectTo: redirectTo})
}

// writeOAuthRequestError não distingue pedido inexistente, vencido ou já decidido.
func writeOAuthRequestError(w http.ResponseWriter, operation string, err error) {
	if errors.Is(err, oauth.ErrRequestNotFound) {
		writeError(w, "request_not_found", "pedido de autorizacao inexistente, vencido ou ja decidido", http.StatusNotFound)
		return
	}
	writeOAuthInternal(w, operation, err)
}
