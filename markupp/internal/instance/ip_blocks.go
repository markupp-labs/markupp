package instance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// Erros do bloqueio de IP da instância (ADR-0061).
var (
	ErrInvalidIPBlock  = errors.New("invalid ip block")
	ErrBlocksOwnIP     = errors.New("the range includes the IP of the administrator who asked for it")
	ErrIPBlockExists   = errors.New("ip range already blocked")
	ErrIPBlockNotFound = errors.New("ip block not found")
)

// ipBlocksReadTimeout limita a leitura da lista, para o banco lento não prender o pedido.
const ipBlocksReadTimeout = 5 * time.Second

// IPBlock é um IP ou faixa que a API recusa antes da autenticação.
type IPBlock struct {
	ID        string
	Prefix    netip.Prefix
	Note      string
	CreatedAt time.Time
}

// IPBlockRequest é o pedido de bloqueio, com quem pede e de onde, para o administrador não
// se trancar fora.
type IPBlockRequest struct {
	ActorID string
	ActorIP string
	CIDR    string
	Note    string
}

// IPBlockStore persiste os bloqueios de IP e audita cada mudança na mesma transação.
type IPBlockStore interface {
	ListIPBlocks(ctx context.Context) ([]IPBlock, error)
	// CreateIPBlock devolve ErrIPBlockExists se a faixa já está bloqueada.
	CreateIPBlock(ctx context.Context, block IPBlock, event auth.AuditEventRecord) error
	// DeleteIPBlock devolve ErrIPBlockNotFound se o bloqueio não existe.
	DeleteIPBlock(ctx context.Context, id string, event auth.AuditEventRecord) error
}

// IPBlockService é a lista de bloqueios que o administrador da instância mantém.
type IPBlockService struct {
	store IPBlockStore
	clock func() time.Time
	newID func() string
}

// NewIPBlockService monta o serviço sobre o repositório, o relógio e o gerador de ids.
func NewIPBlockService(store IPBlockStore, clock func() time.Time, newID func() string) *IPBlockService {
	return &IPBlockService{store: store, clock: clock, newID: newID}
}

// List devolve os bloqueios na ordem de criação.
func (s *IPBlockService) List(ctx context.Context) ([]IPBlock, error) {
	blocks, err := s.store.ListIPBlocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ip blocks: %w", err)
	}
	return blocks, nil
}

// Create bloqueia a faixa do pedido, normalizada, e recusa a que pega o IP de quem pede.
func (s *IPBlockService) Create(ctx context.Context, req IPBlockRequest) (IPBlock, error) {
	prefix, err := parseBlockedRange(req.CIDR)
	if err != nil {
		return IPBlock{}, err
	}
	if actor, err := netip.ParseAddr(req.ActorIP); err == nil && prefix.Contains(actor.Unmap()) {
		return IPBlock{}, fmt.Errorf("%w: %s contains %s", ErrBlocksOwnIP, prefix, req.ActorIP)
	}
	block := IPBlock{ID: s.newID(), Prefix: prefix, Note: strings.TrimSpace(req.Note), CreatedAt: s.clock()}
	event := auth.AuditEventRecord{ID: block.ID, ActorID: req.ActorID, Action: "ip_blocks.create",
		Target: prefix.String(), Result: "success", OccurredAt: block.CreatedAt}
	if err := s.store.CreateIPBlock(ctx, block, event); err != nil {
		return IPBlock{}, fmt.Errorf("block %s: %w", prefix, err)
	}
	return block, nil
}

// parseBlockedRange aceita CIDR ou IP solto, que vira a faixa de um endereço só.
func parseBlockedRange(raw string) (netip.Prefix, error) {
	raw = strings.TrimSpace(raw)
	if addr, err := netip.ParseAddr(raw); err == nil {
		addr = addr.Unmap()
		return netip.PrefixFrom(addr, addr.BitLen()), nil
	}
	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("%w: %q, expected an IP or a CIDR such as 198.51.100.0/24 or 2001:db8::/32", ErrInvalidIPBlock, raw)
	}
	return prefix.Masked(), nil
}

// Delete desbloqueia a faixa id a pedido de actorID.
func (s *IPBlockService) Delete(ctx context.Context, actorID, id string) error {
	event := auth.AuditEventRecord{ID: s.newID(), ActorID: actorID, Action: "ip_blocks.delete",
		Target: id, Result: "success", OccurredAt: s.clock()}
	if err := s.store.DeleteIPBlock(ctx, id, event); err != nil {
		return fmt.Errorf("unblock %q: %w", id, err)
	}
	return nil
}

// RefreshingIPBlocks é a lista de faixas bloqueadas que cada réplica relê depois de cada
// intervalo. Com o banco fora, vale a última lista lida.
type RefreshingIPBlocks struct {
	store  IPBlockStore
	every  time.Duration
	now    func() time.Time
	logger *slog.Logger

	mu        sync.Mutex
	checkedAt time.Time
	prefixes  []netip.Prefix
}

// NewRefreshingIPBlocks monta a lista sobre store, relida no máximo a cada every.
func NewRefreshingIPBlocks(store IPBlockStore, every time.Duration, now func() time.Time, logger *slog.Logger) *RefreshingIPBlocks {
	return &RefreshingIPBlocks{store: store, every: every, now: now, logger: logger}
}

// Blocked diz se addr cai numa faixa bloqueada.
func (r *RefreshingIPBlocks) Blocked(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range r.snapshot() {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (r *RefreshingIPBlocks) snapshot() []netip.Prefix {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.checkedAt.IsZero() && r.now().Sub(r.checkedAt) < r.every {
		return r.prefixes
	}
	r.checkedAt = r.now()
	r.refresh()
	return r.prefixes
}

func (r *RefreshingIPBlocks) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), ipBlocksReadTimeout)
	defer cancel()
	blocks, err := r.store.ListIPBlocks(ctx)
	if err != nil {
		r.logger.Error("ler os bloqueios de IP", "err", err, "faixas_vigentes", len(r.prefixes))
		return
	}
	prefixes := make([]netip.Prefix, 0, len(blocks))
	for _, b := range blocks {
		prefixes = append(prefixes, b.Prefix)
	}
	r.prefixes = prefixes
}
