package auth

import (
	"context"
	"fmt"
	"time"
)

// SessionView é a sessão de login como a conta e o administrador a veem (ADR-0061).
type SessionView struct {
	ID         string    `json:"id"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	// Current marca a sessão do token de quem pede.
	Current bool `json:"current"`
}

// ListSessions lista as sessões ativas de userID, com currentSessionID marcada.
func (s *Service) ListSessions(ctx context.Context, userID, currentSessionID string) ([]SessionView, error) {
	records, err := s.repo.ListActiveSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions of user %q: %w", userID, err)
	}
	views := make([]SessionView, 0, len(records))
	for _, r := range records {
		views = append(views, SessionView{ID: r.ID, IP: r.Origin.IP, UserAgent: r.Origin.UserAgent,
			CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt, Current: r.ID == currentSessionID})
	}
	return views, nil
}

// EndSession encerra a sessão sessionID de userID a pedido de actorID, que é a própria
// conta ou o administrador da instância, e audita quem encerrou.
func (s *Service) EndSession(ctx context.Context, actorID, userID, sessionID string) error {
	if err := s.repo.EndUserSession(ctx, userID, sessionID); err != nil {
		return fmt.Errorf("end session %q of user %q: %w", sessionID, userID, err)
	}
	event := AuditEventRecord{ID: s.newID(), ActorID: actorID, Action: "auth.session_end",
		Target: sessionID, Result: "success", OccurredAt: s.clock()}
	if err := s.repo.RecordAuditEvent(ctx, event); err != nil {
		return fmt.Errorf("audit end of session %q: %w", sessionID, err)
	}
	return nil
}
