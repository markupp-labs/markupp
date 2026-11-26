package storage

import (
	"context"
	"fmt"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// ListActiveSessions lista as sessões de userID que ainda renovam, do último uso para trás.
func (r *PostgresAuthRepository) ListActiveSessions(ctx context.Context, userID string) ([]auth.SessionRecord, error) {
	rows, err := r.q.ListActiveUserSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active sessions of user %q: %w", userID, err)
	}
	records := make([]auth.SessionRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, auth.SessionRecord{ID: row.ID, UserID: row.UserID, CreatedAt: row.CreatedAt,
			LastUsedAt: row.LastUsedAt, Origin: auth.Origin{IP: row.ClientIp, UserAgent: row.UserAgent}})
	}
	return records, nil
}

// EndUserSession encerra a sessão de userID e revoga os refresh dela. Sessão inexistente,
// já encerrada ou de outra conta dá auth.ErrSessionNotFound.
func (r *PostgresAuthRepository) EndUserSession(ctx context.Context, userID, sessionID string) error {
	ended, err := r.q.EndUserSession(ctx, gen.EndUserSessionParams{ID: sessionID, UserID: userID})
	if err != nil {
		return fmt.Errorf("end session %q of user %q: %w", sessionID, userID, err)
	}
	if ended == 0 {
		return auth.ErrSessionNotFound
	}
	return nil
}
