package instance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// Faixa e padrão do prazo de guarda do histórico e do IP, em dias (ADR-0061).
const (
	minRetentionDays     = 1
	maxRetentionDays     = 3650
	defaultRetentionDays = 365
	// cleanupEvery é o intervalo mínimo entre duas limpezas, somando todas as réplicas.
	cleanupEvery = 24 * time.Hour
)

// ErrInvalidRetention marca o prazo fora da faixa.
var ErrInvalidRetention = errors.New("invalid retention")

// RetentionView é o prazo vigente e se quem hospeda o fixou.
type RetentionView struct {
	Days       int  `json:"days"`
	HostLocked bool `json:"host_locked"`
}

// RetentionCleanup é uma limpeza: apaga o que é de antes de Cutoff, se a última limpeza
// foi antes de DueAfter.
type RetentionCleanup struct {
	Cutoff   time.Time
	Now      time.Time
	DueAfter time.Time
}

// RetentionResult diz se a limpeza rodou e quanto ela apagou.
type RetentionResult struct {
	Ran         bool
	AuditEvents int64
	Origins     int64
}

// RetentionStore persiste a escolha do prazo e roda a limpeza.
type RetentionStore interface {
	// LoadRetentionDays devolve a escolha do administrador, zero sem escolha.
	LoadRetentionDays(ctx context.Context) (int, error)
	SaveRetentionDays(ctx context.Context, days int, event auth.AuditEventRecord) error
	// CleanupIfDue limpa só a réplica que pegar a trava e só se a limpeza estiver vencida.
	CleanupIfDue(ctx context.Context, c RetentionCleanup) (RetentionResult, error)
}

// RetentionService é o prazo de guarda que o administrador escolhe e quem hospeda pode fixar.
type RetentionService struct {
	store    RetentionStore
	hostDays int
	clock    func() time.Time
	newID    func() string
}

// NewRetentionService monta o serviço; hostDays diferente de zero fixa o prazo.
func NewRetentionService(store RetentionStore, hostDays int, clock func() time.Time, newID func() string) *RetentionService {
	return &RetentionService{store: store, hostDays: hostDays, clock: clock, newID: newID}
}

// Settings devolve o prazo vigente: o de quem hospeda, o do administrador ou o padrão.
func (s *RetentionService) Settings(ctx context.Context) (RetentionView, error) {
	if s.hostDays != 0 {
		return RetentionView{Days: s.hostDays, HostLocked: true}, nil
	}
	days, err := s.store.LoadRetentionDays(ctx)
	if err != nil {
		return RetentionView{}, fmt.Errorf("load retention: %w", err)
	}
	if days == 0 {
		days = defaultRetentionDays
	}
	return RetentionView{Days: days}, nil
}

// Update guarda a escolha do administrador actorID.
func (s *RetentionService) Update(ctx context.Context, actorID string, days int) (RetentionView, error) {
	if s.hostDays != 0 {
		return RetentionView{}, fmt.Errorf("%w: retention fixed at %d days", ErrHostLocked, s.hostDays)
	}
	if days < minRetentionDays || days > maxRetentionDays {
		return RetentionView{}, fmt.Errorf("%w: %d days, expected from %d to %d", ErrInvalidRetention, days, minRetentionDays, maxRetentionDays)
	}
	event := auth.AuditEventRecord{ID: s.newID(), ActorID: actorID, Action: "instance.retention_update",
		Target: strconv.Itoa(days), Result: "success", OccurredAt: s.clock()}
	if err := s.store.SaveRetentionDays(ctx, days, event); err != nil {
		return RetentionView{}, fmt.Errorf("save retention of %d days: %w", days, err)
	}
	return RetentionView{Days: days}, nil
}

// CleanupIfDue apaga o histórico e tira o IP das sessões encerradas antes do prazo, se a
// limpeza estiver vencida, e registra no log o que apagou.
func (s *RetentionService) CleanupIfDue(ctx context.Context, logger *slog.Logger) error {
	view, err := s.Settings(ctx)
	if err != nil {
		return err
	}
	now := s.clock()
	result, err := s.store.CleanupIfDue(ctx, RetentionCleanup{Cutoff: now.AddDate(0, 0, -view.Days), Now: now, DueAfter: now.Add(-cleanupEvery)})
	if err != nil {
		return fmt.Errorf("clean up history older than %d days: %w", view.Days, err)
	}
	if result.Ran {
		logger.Info("limpeza do historico", "dias", view.Days, "eventos_apagados", result.AuditEvents, "origens_apagadas", result.Origins)
	}
	return nil
}

// RunRetentionCleanup tenta a limpeza a cada every até ctx acabar; falha vai para o log e
// a próxima tentativa segue.
func RunRetentionCleanup(ctx context.Context, svc *RetentionService, every time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		if err := svc.CleanupIfDue(ctx, logger); err != nil {
			logger.Error("limpeza do historico", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
