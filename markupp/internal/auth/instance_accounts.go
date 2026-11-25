package auth

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrCannotTargetSelf recusa o administrador bloquear ou apagar a própria conta.
	ErrCannotTargetSelf = errors.New("the instance administrator cannot block or delete their own account")
	// ErrInvalidAccountQuery marca o filtro de contas inválido.
	ErrInvalidAccountQuery = errors.New("invalid account query")
)

// Situações da conta que a lista filtra.
const (
	AccountStatusActive  = "active"
	AccountStatusBlocked = "blocked"
)

const defaultAccountPageSize = 50

// AccountQuery filtra a lista de contas da instância: Email é parte do email, sem
// diferenciar maiúsculas, e Status é vazio, active ou blocked.
type AccountQuery struct {
	Email  string
	Status string
	Offset int
	Limit  int
}

// AccountSummary é a conta como o administrador da instância a vê na lista (ADR-0061).
type AccountSummary struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	// LoginMethods são os jeitos de entrar: local para senha e o id de cada provedor.
	LoginMethods []string  `json:"login_methods"`
	CreatedAt    time.Time `json:"created_at"`
	// LastAccessAt é o último uso de sessão ou app autorizado; nulo sem nenhum.
	LastAccessAt *time.Time `json:"last_access_at"`
	Blocked      bool       `json:"blocked"`
}

// OwnedTenant é o tenant que some com a conta, por ela ser a única owner.
type OwnedTenant struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Notes int64  `json:"notes"`
}

// AccountDetail é a conta com o que apagá-la leva junto.
type AccountDetail struct {
	AccountSummary
	SoleOwnerTenants []OwnedTenant `json:"sole_owner_tenants"`
}

// InstanceAccountsRepository persiste a gestão das contas pelo administrador da instância.
type InstanceAccountsRepository interface {
	ListAccounts(ctx context.Context, q AccountQuery) ([]AccountSummary, error)
	// GetAccountDetail devolve ErrUserNotFound se a conta não existe.
	GetAccountDetail(ctx context.Context, id string) (AccountDetail, error)
	// SetAccountBlocked bloqueia ou desbloqueia e audita, numa transação; bloquear também
	// encerra as sessões e revoga os apps autorizados. ErrUserNotFound sem a conta.
	SetAccountBlocked(ctx context.Context, id string, blocked bool, event AuditEventRecord) error
	// DeleteAccount apaga a conta e os tenants de que ela é a única owner, e audita, numa
	// transação. ErrUserNotFound sem a conta.
	DeleteAccount(ctx context.Context, id string, event AuditEventRecord) error
}

// InstanceAccounts é a gestão das contas pelo administrador da instância.
type InstanceAccounts struct {
	repo  InstanceAccountsRepository
	clock func() time.Time
	newID func() string
}

// NewInstanceAccounts monta o serviço sobre o repositório, o relógio e o gerador de ids.
func NewInstanceAccounts(repo InstanceAccountsRepository, clock func() time.Time, newID func() string) *InstanceAccounts {
	return &InstanceAccounts{repo: repo, clock: clock, newID: newID}
}

// List lista as contas pelo filtro; sem limite, a página tem defaultAccountPageSize.
func (s *InstanceAccounts) List(ctx context.Context, q AccountQuery) ([]AccountSummary, error) {
	if q.Status != "" && q.Status != AccountStatusActive && q.Status != AccountStatusBlocked {
		return nil, fmt.Errorf("%w: status %q, expected %s or %s", ErrInvalidAccountQuery, q.Status, AccountStatusActive, AccountStatusBlocked)
	}
	if q.Limit == 0 {
		q.Limit = defaultAccountPageSize
	}
	accounts, err := s.repo.ListAccounts(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list instance accounts: %w", err)
	}
	return accounts, nil
}

// Get devolve a conta com o que apagá-la leva junto.
func (s *InstanceAccounts) Get(ctx context.Context, id string) (AccountDetail, error) {
	detail, err := s.repo.GetAccountDetail(ctx, id)
	if err != nil {
		return AccountDetail{}, fmt.Errorf("get instance account %q: %w", id, err)
	}
	return detail, nil
}

// SetBlocked bloqueia ou desbloqueia a conta id a pedido de actorID.
func (s *InstanceAccounts) SetBlocked(ctx context.Context, actorID, id string, blocked bool) error {
	if actorID == id {
		return ErrCannotTargetSelf
	}
	action := "accounts.unblock"
	if blocked {
		action = "accounts.block"
	}
	if err := s.repo.SetAccountBlocked(ctx, id, blocked, s.event(actorID, action, id)); err != nil {
		return fmt.Errorf("set blocked=%t on account %q: %w", blocked, id, err)
	}
	return nil
}

// Delete apaga a conta id a pedido de actorID, com os tenants de que ela é a única owner.
func (s *InstanceAccounts) Delete(ctx context.Context, actorID, id string) error {
	if actorID == id {
		return ErrCannotTargetSelf
	}
	if err := s.repo.DeleteAccount(ctx, id, s.event(actorID, "accounts.delete", id)); err != nil {
		return fmt.Errorf("delete account %q: %w", id, err)
	}
	return nil
}

func (s *InstanceAccounts) event(actorID, action, target string) AuditEventRecord {
	return AuditEventRecord{ID: s.newID(), ActorID: actorID, Action: action, Target: target,
		Result: "success", OccurredAt: s.clock()}
}
