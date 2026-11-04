package api

import (
	"net/http"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

type notificationResponse struct {
	Kind       string    `json:"kind"`
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	TenantName string    `json:"tenant_name"`
	Role       string    `json:"role"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type notificationFeedResponse struct {
	Items  []notificationResponse `json:"items"`
	Unread int                    `json:"unread"`
}

// myNotifications lista o que pede a atenção da conta (ADR-0049). Traz os convites
// pendentes, os mesmos de GET /me/invites, então o token OAuth precisa de administracao.
func (h *authHandler) myNotifications(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	feed, err := h.svc.ListMyNotifications(r.Context(), actor.UserID)
	if err != nil {
		writeLoggedInternal(w, "falha nas notificacoes da conta", "listar notificacoes da conta", err)
		return
	}
	writeJSON(w, http.StatusOK, toNotificationFeedResponse(feed))
}

func toNotificationFeedResponse(feed auth.NotificationFeed) notificationFeedResponse {
	items := make([]notificationResponse, 0, len(feed.Items))
	for _, item := range feed.Items {
		items = append(items, notificationResponse(item))
	}
	return notificationFeedResponse{Items: items, Unread: feed.Unread}
}
