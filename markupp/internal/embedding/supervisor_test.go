package embedding_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
)

// FakeSettingSource devolve a escolha atual, que o teste troca entre as voltas.
type FakeSettingSource struct {
	setting embedding.Setting
	err     error
}

func (f *FakeSettingSource) Current(context.Context) (embedding.Setting, error) {
	return f.setting, f.err
}

// FakeStatusReporter guarda cada informe, na ordem.
type FakeStatusReporter struct {
	hosts  []string
	states []string
	errors []string
}

func (f *FakeStatusReporter) ReportEmbeddingHost(_ context.Context, locked bool, provider string) error {
	if locked {
		provider = "travado:" + provider
	}
	f.hosts = append(f.hosts, provider)
	return nil
}

func (f *FakeStatusReporter) ReportModelStatus(_ context.Context, state, message string) error {
	f.states, f.errors = append(f.states, state), append(f.errors, message)
	return nil
}

// FakeProviderOpener monta o fakeEmbeddingProvider para todo provedor menos none, e
// conta aberturas e fechamentos.
type FakeProviderOpener struct {
	opened []embedding.Setting
	closed int
	err    error
}

func (f *FakeProviderOpener) Open(_ context.Context, s embedding.Setting) (embedding.Provider, func() error, error) {
	f.opened = append(f.opened, s)
	if f.err != nil {
		return nil, nil, f.err
	}
	closeFn := func() error { f.closed++; return nil }
	if s.Provider == "none" {
		return nil, closeFn, nil
	}
	return &fakeEmbeddingProvider{}, closeFn, nil
}

// FakeModelRegistry guarda os modelos registrados.
type FakeModelRegistry struct{ models []string }

func (f *FakeModelRegistry) Use(_ context.Context, name string, dimension int) (bool, error) {
	f.models = append(f.models, name)
	return true, nil
}

// FakeSupervisorClock é o relógio do supervisor, avançado à mão pelo teste.
type FakeSupervisorClock struct{ now time.Time }

func (c *FakeSupervisorClock) Now() time.Time { return c.now }

func (c *FakeSupervisorClock) advance(d time.Duration) { c.now = c.now.Add(d) }

// retryBaseDeTeste é a primeira espera depois de uma falha nos testes.
const retryBaseDeTeste = 30 * time.Second

type supervisorDeTeste struct {
	clock    *FakeSupervisorClock
	source   *FakeSettingSource
	status   *FakeStatusReporter
	opener   *FakeProviderOpener
	registry *FakeModelRegistry
	queue    *fakeEmbeddingQueue
	sup      *embedding.Supervisor
}

func novoSupervisor(setting embedding.Setting, locked bool) supervisorDeTeste {
	d := supervisorDeTeste{clock: &FakeSupervisorClock{now: time.Unix(1_000_000, 0)}, source: &FakeSettingSource{setting: setting}, status: &FakeStatusReporter{},
		opener: &FakeProviderOpener{}, registry: &FakeModelRegistry{}, queue: &fakeEmbeddingQueue{}}
	d.sup = embedding.NewSupervisor(embedding.SupervisorDeps{
		Source: d.source, Status: d.status, Open: d.opener.Open, Use: d.registry.Use, Queue: d.queue,
		HostLocked: locked, HostProvider: setting.Provider, ChunkTokens: 2, BatchSize: 16, Logger: slog.New(slog.DiscardHandler),
		Now: d.clock.Now, RetryBase: retryBaseDeTeste,
	})
	return d
}

func TestRefresh_ProvedorLocal_InformaBaixandoEDepoisPronto(t *testing.T) {
	d := novoSupervisor(embedding.Setting{Provider: "local"}, false)

	d.sup.Refresh(context.Background())

	assert.Equal(t, []string{"downloading", "ready"}, d.status.states)
	assert.Equal(t, []string{"fake"}, d.registry.models)
	assert.Equal(t, []string{""}, d.status.hosts, "sem trava, nenhum provedor de quem hospeda")
}

func TestRefresh_SemProvedor_RegistraNoneSemEstado(t *testing.T) {
	d := novoSupervisor(embedding.Setting{Provider: "none"}, false)

	d.sup.Refresh(context.Background())

	assert.Equal(t, []string{"none"}, d.registry.models)
	assert.Equal(t, []string{""}, d.status.states)
}

func TestRefresh_MesmaEscolha_NaoReabreOProvedor(t *testing.T) {
	d := novoSupervisor(embedding.Setting{Provider: "openai-compat", URL: "u", Model: "m"}, false)

	d.sup.Refresh(context.Background())
	d.sup.Refresh(context.Background())

	assert.Len(t, d.opener.opened, 1)
	assert.Equal(t, []string{"ready"}, d.status.states)
}

func TestRefresh_EscolhaMudou_FechaOAnteriorEAbreONovo(t *testing.T) {
	d := novoSupervisor(embedding.Setting{Provider: "openai-compat", URL: "u", Model: "m"}, false)
	d.sup.Refresh(context.Background())
	d.source.setting = embedding.Setting{Provider: "local", Revision: time.Unix(2, 0)}

	d.sup.Refresh(context.Background())

	assert.Len(t, d.opener.opened, 2)
	assert.Equal(t, 1, d.opener.closed)
	assert.Equal(t, "local", d.opener.opened[1].Provider)
}

func TestRefresh_MesmaEscolhaGravadaDeNovo_TentaOutraVez(t *testing.T) {
	d := novoSupervisor(embedding.Setting{Provider: "local", Revision: time.Unix(1, 0)}, false)
	d.opener.err = errors.New("sem rede")
	d.sup.Refresh(context.Background())
	d.opener.err = nil
	d.source.setting.Revision = time.Unix(2, 0)

	d.sup.Refresh(context.Background())

	assert.Equal(t, []string{"downloading", "error", "downloading", "ready"}, d.status.states)
}

func TestRefresh_FalhouComAMesmaEscolha_TentaDeNovoNaProximaVolta(t *testing.T) {
	d := novoSupervisor(embedding.Setting{Provider: "local"}, true)
	d.opener.err = errors.New("sem rede")
	d.sup.Refresh(context.Background())
	d.opener.err = nil
	d.clock.advance(retryBaseDeTeste)

	d.sup.Refresh(context.Background())

	assert.Len(t, d.opener.opened, 2)
	assert.Equal(t, []string{"downloading", "error", "downloading", "ready"}, d.status.states)
}

func TestRefresh_FalhaAoAbrir_InformaErroComAMensagem(t *testing.T) {
	d := novoSupervisor(embedding.Setting{Provider: "local"}, false)
	d.opener.err = errors.New("hash diferente")

	d.sup.Refresh(context.Background())

	assert.Equal(t, []string{"downloading", "error"}, d.status.states)
	assert.Equal(t, "hash diferente", d.status.errors[1])
}

func TestRefresh_Travado_InformaOProvedorDeQuemHospeda(t *testing.T) {
	d := novoSupervisor(embedding.Setting{Provider: "none"}, true)

	d.sup.Refresh(context.Background())

	assert.Equal(t, []string{"travado:none"}, d.status.hosts)
}

func TestRefresh_FalhaAoLerAEscolha_MantemOProvedorAtual(t *testing.T) {
	d := novoSupervisor(embedding.Setting{Provider: "openai-compat", URL: "u", Model: "m"}, false)
	d.sup.Refresh(context.Background())
	d.source.err = errors.New("banco fora")

	d.sup.Refresh(context.Background())

	assert.Len(t, d.opener.opened, 1)
	assert.Zero(t, d.opener.closed)
}

// cancelaQuandoVazia encerra o laço na primeira vez que a fila fica vazia depois
// de entregar os lotes.
type cancelaQuandoVazia struct {
	*fakeEmbeddingQueue
	cancel context.CancelFunc
}

func (c cancelaQuandoVazia) NextBatch(ctx context.Context, limit int) (embedding.PendingBatch, bool, error) {
	batch, found, err := c.fakeEmbeddingQueue.NextBatch(ctx, limit)
	if !found {
		c.cancel()
	}
	return batch, found, err
}

func TestRun_ComProvedor_RefazOsVetoresPendentesEParaNoCancelamento(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fila := &fakeEmbeddingQueue{batches: []embedding.PendingBatch{{TenantID: "t", VaultID: "v", Notes: []embedding.PendingNote{{NoteID: "a", Content: "xx"}}}}}
	opener := &FakeProviderOpener{}
	sup := embedding.NewSupervisor(embedding.SupervisorDeps{
		Source: &FakeSettingSource{setting: embedding.Setting{Provider: "local"}}, Status: &FakeStatusReporter{},
		Open: opener.Open, Use: (&FakeModelRegistry{}).Use, Queue: cancelaQuandoVazia{fila, cancel},
		ChunkTokens: 2, BatchSize: 16, Logger: slog.New(slog.DiscardHandler),
	})

	err := sup.Run(ctx, time.Hour)

	require.NoError(t, err)
	assert.Len(t, fila.saved, 1)
	assert.Equal(t, 1, opener.closed, "o encerramento fecha o provedor")
}
