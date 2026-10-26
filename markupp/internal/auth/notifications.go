package auth

import (
	"context"
	"fmt"
	"time"
)

// NotificationKindInvite é a notificação de um convite pendente para a conta (ADR-0049).
const NotificationKindInvite = "invite"

// Notification é o que pede a atenção da conta. Kind diz a fonte; os demais campos são
// os do convite, hoje a única fonte.
type Notification struct {
	Kind       string
	ID         string
	TenantID   string
	TenantName string
	Role       string
	ExpiresAt  time.Time
}

// NotificationFeed é a lista da conta e quantas pedem resposta. Sem estado de lido, o
// Unread conta os itens ainda pendentes.
type NotificationFeed struct {
	Items  []Notification
	Unread int
}

// ListMyNotifications deriva as notificações da conta userID dos convites pendentes, na
// leitura, sem tabela própria (ADR-0049).
func (s *Service) ListMyNotifications(ctx context.Context, userID string) (NotificationFeed, error) {
	invites, err := s.repo.ListPendingInvites(ctx, userID, s.clock())
	if err != nil {
		return NotificationFeed{}, fmt.Errorf("listar notificacoes da conta %q: %w", userID, err)
	}
	items := make([]Notification, 0, len(invites))
	for _, inv := range invites {
		items = append(items, inviteNotification(inv))
	}
	return NotificationFeed{Items: items, Unread: len(items)}, nil
}

func inviteNotification(inv PendingInvite) Notification {
	return Notification{
		Kind: NotificationKindInvite, ID: inv.ID, TenantID: inv.TenantID,
		TenantName: inv.TenantName, Role: inv.Role, ExpiresAt: inv.ExpiresAt,
	}
}
