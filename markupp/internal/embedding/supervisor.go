package embedding

import (
	"context"
	"log/slog"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/drain"
)

// Provedores que o supervisor trata de um jeito próprio; os nomes são os de config.
const (
	providerNone  = "none"
	providerLocal = "local"
)

// Estados do modelo informados ao banco; vazio é a instalação sem provedor.
const (
	stateDownloading = "downloading"
	stateReady       = "ready"
	stateError       = "error"
)

// Setting é o provedor a rodar. Revision muda a cada gravação da escolha, e outra
// revisão reabre o provedor mesmo com os mesmos valores.
type Setting struct {
	Provider string
	URL      string
	Model    string
	APIKey   string
	Revision time.Time
}

// SettingSource diz o provedor a rodar: o de quem hospeda ou o escolhido no banco.
type SettingSource interface {
	Current(ctx context.Context) (Setting, error)
}

// StatusReporter recebe a trava de quem hospeda e o estado do modelo (ADR-0057).
type StatusReporter interface {
	ReportEmbeddingHost(ctx context.Context, locked bool, provider string) error
	ReportModelStatus(ctx context.Context, state, message string) error
}

// ProviderOpener monta o provedor de setting, nil sem provedor, e a função que o fecha.
type ProviderOpener func(ctx context.Context, setting Setting) (Provider, func() error, error)

// SupervisorDeps é o que o supervisor usa. HostLocked e HostProvider são o que ele
// informa a cada volta sobre a configuração de quem hospeda.
type SupervisorDeps struct {
	Source       SettingSource
	Status       StatusReporter
	Open         ProviderOpener
	Use          ModelRegistry
	Queue        Queue
	HostLocked   bool
	HostProvider string
	ChunkTokens  int
	BatchSize    int
	Logger       *slog.Logger
}

// Supervisor roda o indexador de embedding com o provedor escolhido e o troca, sem
// reiniciar o processo, quando a escolha muda.
type Supervisor struct {
	deps          SupervisorDeps
	current       *Setting
	indexer       *Indexer
	closeProvider func() error
}

// NewSupervisor monta o supervisor, ainda sem provedor aberto.
func NewSupervisor(deps SupervisorDeps) *Supervisor {
	return &Supervisor{deps: deps}
}

// Run aplica a escolha, refaz os vetores pendentes e, com a fila vazia, espera interval
// e relê a escolha. Termina sem erro no cancelamento, fechando o provedor.
func (s *Supervisor) Run(ctx context.Context, interval time.Duration) error {
	defer s.stop()
	s.Refresh(ctx)
	wait := drain.WaitInterval(interval)
	err := drain.Run(ctx, s.next, func(ctx context.Context) error {
		if err := wait(ctx); err != nil {
			return err
		}
		s.Refresh(ctx)
		return nil
	})
	if ctx.Err() != nil {
		return nil
	}
	return err
}

// Refresh informa a trava de quem hospeda e, quando a escolha mudou, troca o provedor.
// A falha de leitura fica no log e mantém o provedor atual.
func (s *Supervisor) Refresh(ctx context.Context) {
	if err := s.deps.Status.ReportEmbeddingHost(ctx, s.deps.HostLocked, s.hostProvider()); err != nil {
		s.deps.Logger.Error("informar a trava do provedor de embedding", "err", err)
	}
	setting, err := s.deps.Source.Current(ctx)
	if err != nil {
		s.deps.Logger.Error("ler a escolha do provedor de embedding", "err", err)
		return
	}
	if s.current != nil && *s.current == setting {
		return
	}
	s.switchTo(ctx, setting)
}

func (s *Supervisor) hostProvider() string {
	if !s.deps.HostLocked {
		return ""
	}
	return s.deps.HostProvider
}

// switchTo fecha o provedor atual e abre o de setting, informando o estado do modelo.
func (s *Supervisor) switchTo(ctx context.Context, setting Setting) {
	s.stop()
	s.current = &setting
	if setting.Provider == providerLocal {
		s.report(ctx, stateDownloading, "")
	}
	provider, closeProvider, err := s.deps.Open(ctx, setting)
	if err != nil {
		s.fail(ctx, setting, err)
		return
	}
	s.closeProvider = closeProvider
	if provider == nil {
		s.activateNone(ctx)
		return
	}
	s.activate(ctx, setting, provider)
}

// activateNone registra a instalação sem modelo, o que apaga os vetores e as ligações
// por sentido de um modelo anterior.
func (s *Supervisor) activateNone(ctx context.Context) {
	if _, err := s.deps.Use(ctx, providerNone, 0); err != nil {
		s.deps.Logger.Error("registrar a instalacao sem modelo de embedding", "err", err)
	}
	s.report(ctx, "", "")
	s.deps.Logger.Warn("indexador de embedding ocioso", "motivo", "sem provedor, sem ligacao por sentido")
}

func (s *Supervisor) activate(ctx context.Context, setting Setting, provider Provider) {
	changed, err := Activate(ctx, provider, s.deps.Use)
	if err != nil {
		s.fail(ctx, setting, err)
		return
	}
	s.indexer = NewIndexer(s.deps.Queue, provider, s.deps.ChunkTokens, s.deps.BatchSize, s.deps.Logger)
	s.report(ctx, stateReady, "")
	s.deps.Logger.Info("indexador de embedding em laco", "modelo", provider.Name(), "modelo_trocado", changed)
}

// fail informa o erro e esquece a escolha, para a próxima volta tentar abrir de novo
// mesmo quando quem hospeda trava o provedor e a escolha nunca muda.
func (s *Supervisor) fail(ctx context.Context, setting Setting, err error) {
	s.stop()
	s.current = nil
	s.report(ctx, stateError, err.Error())
	s.deps.Logger.Error("abrir o provedor de embedding", "provedor", setting.Provider, "err", err)
}

func (s *Supervisor) report(ctx context.Context, state, message string) {
	if err := s.deps.Status.ReportModelStatus(ctx, state, message); err != nil {
		s.deps.Logger.Error("informar o estado do modelo de embedding", "estado", state, "err", err)
	}
}

// next refaz o próximo lote. O erro de um lote vai para o log e espera o intervalo,
// como no indexador de texto.
func (s *Supervisor) next(ctx context.Context) (bool, error) {
	if s.indexer == nil {
		return false, nil
	}
	found, err := s.indexer.IndexNext(ctx)
	if err != nil {
		s.deps.Logger.Error("refazer vetores do proximo lote", "err", err)
		return false, nil
	}
	return found, nil
}

// stop fecha o provedor aberto, se há, e esquece o indexador.
func (s *Supervisor) stop() {
	s.indexer = nil
	if s.closeProvider == nil {
		return
	}
	if err := s.closeProvider(); err != nil {
		s.deps.Logger.Error("fechar o provedor de embedding", "err", err)
	}
	s.closeProvider = nil
}
