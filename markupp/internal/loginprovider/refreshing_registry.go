package loginprovider

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// sourceTimeout limita a leitura da lista, para o banco lento não prender o login.
const sourceTimeout = 5 * time.Second

// Source dá os provedores vigentes e uma revisão que muda quando eles mudam (ADR-0060).
type Source interface {
	LoginProviders(ctx context.Context) (revision string, settings []Settings, err error)
}

// RefreshingRegistry é a lista de provedores que relê a fonte depois de cada intervalo e
// se refaz quando a revisão muda. Com a fonte fora, vale a última lista lida.
type RefreshingRegistry struct {
	source Source
	client *http.Client
	every  time.Duration
	now    func() time.Time
	logger *slog.Logger

	mu        sync.Mutex
	checkedAt time.Time
	revision  string
	current   *Registry
}

// NewRefreshingRegistry monta a lista sobre source, relida no máximo a cada every.
func NewRefreshingRegistry(source Source, client *http.Client, every time.Duration, now func() time.Time, logger *slog.Logger) *RefreshingRegistry {
	return &RefreshingRegistry{source: source, client: client, every: every, now: now, logger: logger, current: &Registry{}}
}

// List devolve os provedores vigentes.
func (r *RefreshingRegistry) List() []Provider { return r.snapshot().List() }

// Get devolve o provedor vigente de id, ou false sem ele.
func (r *RefreshingRegistry) Get(id string) (Provider, bool) { return r.snapshot().Get(id) }

func (r *RefreshingRegistry) snapshot() *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.checkedAt.IsZero() && r.now().Sub(r.checkedAt) < r.every {
		return r.current
	}
	r.checkedAt = r.now()
	r.refresh()
	return r.current
}

// refresh lê a fonte e troca a lista quando a revisão mudou.
func (r *RefreshingRegistry) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), sourceTimeout)
	defer cancel()
	revision, settings, err := r.source.LoginProviders(ctx)
	if err != nil {
		r.logger.Error("ler os provedores de login", "err", err, "revisao_vigente", r.revision)
		return
	}
	if revision != r.revision {
		r.current, r.revision = r.current.rebuilt(settings, r.client), revision
	}
}
