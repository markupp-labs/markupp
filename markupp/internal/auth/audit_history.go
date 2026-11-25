package auth

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrInvalidAuditQuery marca o filtro do histórico inválido.
var ErrInvalidAuditQuery = errors.New("invalid audit query")

const defaultAuditPageSize = 50

// auditResults são os resultados que um evento pode ter.
var auditResults = map[string]bool{"success": true, "failure": true, "blocked": true}

// AuditQuery filtra o histórico: Account é o autor, o período é [Since, Until), e o
// filtro vazio vale tudo.
type AuditQuery struct {
	Account string
	Action  string
	Result  string
	IP      string
	Since   *time.Time
	Until   *time.Time
	Offset  int
	Limit   int
}

// AuditEntry é um evento do histórico de acessos e mudanças sensíveis (ADR-0061).
type AuditEntry struct {
	ID         string    `json:"id"`
	ActorID    string    `json:"actor_id"`
	ActorEmail string    `json:"actor_email"`
	Action     string    `json:"action"`
	Target     string    `json:"target"`
	Result     string    `json:"result"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	OccurredAt time.Time `json:"occurred_at"`
}

// AuditHistoryRepository lê o histórico, do evento mais novo para trás.
type AuditHistoryRepository interface {
	ListAuditEvents(ctx context.Context, q AuditQuery) ([]AuditEntry, error)
}

// AuditHistory é a leitura do histórico pelo administrador da instância.
type AuditHistory struct {
	repo AuditHistoryRepository
}

// NewAuditHistory monta a leitura sobre o repositório.
func NewAuditHistory(repo AuditHistoryRepository) *AuditHistory {
	return &AuditHistory{repo: repo}
}

// List lê o histórico pelo filtro; sem limite, a página tem defaultAuditPageSize.
func (h *AuditHistory) List(ctx context.Context, q AuditQuery) ([]AuditEntry, error) {
	if err := checkAuditQuery(q); err != nil {
		return nil, err
	}
	if q.Limit == 0 {
		q.Limit = defaultAuditPageSize
	}
	entries, err := h.repo.ListAuditEvents(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	return entries, nil
}

func checkAuditQuery(q AuditQuery) error {
	if q.Result != "" && !auditResults[q.Result] {
		return fmt.Errorf("%w: result %q, expected success, failure or blocked", ErrInvalidAuditQuery, q.Result)
	}
	if q.Since != nil && q.Until != nil && !q.Since.Before(*q.Until) {
		return fmt.Errorf("%w: since %s is not before until %s, expected since < until",
			ErrInvalidAuditQuery, q.Since.Format(time.RFC3339), q.Until.Format(time.RFC3339))
	}
	return nil
}
