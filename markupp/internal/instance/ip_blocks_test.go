package instance_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
)

// fakeIPBlockStore guarda os bloqueios em memória, na ordem de criação.
type fakeIPBlockStore struct {
	blocks  []instance.IPBlock
	events  []auth.AuditEventRecord
	listErr error
	reads   int
}

func (f *fakeIPBlockStore) ListIPBlocks(context.Context) ([]instance.IPBlock, error) {
	f.reads++
	return append([]instance.IPBlock{}, f.blocks...), f.listErr
}

func (f *fakeIPBlockStore) CreateIPBlock(_ context.Context, b instance.IPBlock, event auth.AuditEventRecord) error {
	for _, existing := range f.blocks {
		if existing.Prefix == b.Prefix {
			return instance.ErrIPBlockExists
		}
	}
	f.blocks, f.events = append(f.blocks, b), append(f.events, event)
	return nil
}

func (f *fakeIPBlockStore) DeleteIPBlock(_ context.Context, id string, event auth.AuditEventRecord) error {
	for i, b := range f.blocks {
		if b.ID == id {
			f.blocks, f.events = append(f.blocks[:i], f.blocks[i+1:]...), append(f.events, event)
			return nil
		}
	}
	return instance.ErrIPBlockNotFound
}

var instanteDoBloqueio = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func servicoDeBloqueios() (*fakeIPBlockStore, *instance.IPBlockService) {
	store := &fakeIPBlockStore{}
	return store, instance.NewIPBlockService(store, func() time.Time { return instanteDoBloqueio }, func() string { return "b1" })
}

func pedidoDoAdministrador() instance.IPBlockRequest {
	return instance.IPBlockRequest{ActorID: "admin", ActorIP: "203.0.113.7", CIDR: "198.51.100.77/24", Note: "varredura"}
}

func TestIPBlocks_Criar_NormalizaAFaixaEAudita(t *testing.T) {
	store, svc := servicoDeBloqueios()

	block, err := svc.Create(context.Background(), pedidoDoAdministrador())

	require.NoError(t, err)
	assert.Equal(t, instance.IPBlock{ID: "b1", Prefix: netip.MustParsePrefix("198.51.100.0/24"), Note: "varredura",
		CreatedAt: instanteDoBloqueio}, block)
	assert.Equal(t, auth.AuditEventRecord{ID: "b1", ActorID: "admin", Action: "ip_blocks.create",
		Target: "198.51.100.0/24", Result: "success", OccurredAt: instanteDoBloqueio}, store.events[0])
}

func TestIPBlocks_CriarIPSolto_ViraFaixaDeUmEndereco(t *testing.T) {
	casos := map[string]string{"198.51.100.9": "198.51.100.9/32", "2001:db8::1": "2001:db8::1/128"}
	for entrada, faixa := range casos {
		_, svc := servicoDeBloqueios()
		req := pedidoDoAdministrador()
		req.CIDR = entrada

		block, err := svc.Create(context.Background(), req)

		require.NoError(t, err, entrada)
		assert.Equal(t, faixa, block.Prefix.String())
	}
}

func TestIPBlocks_CriarInvalido_RecusaDizendoOFormato(t *testing.T) {
	_, svc := servicoDeBloqueios()
	req := pedidoDoAdministrador()
	req.CIDR = "rede-da-empresa"

	_, err := svc.Create(context.Background(), req)

	require.ErrorIs(t, err, instance.ErrInvalidIPBlock)
	assert.Contains(t, err.Error(), `"rede-da-empresa"`)
	assert.Contains(t, err.Error(), "expected")
}

func TestIPBlocks_FaixaComOIPDoAdministrador_Recusa(t *testing.T) {
	store, svc := servicoDeBloqueios()
	req := pedidoDoAdministrador()
	req.CIDR = "0.0.0.0/0"

	_, err := svc.Create(context.Background(), req)

	require.ErrorIs(t, err, instance.ErrBlocksOwnIP)
	assert.Empty(t, store.blocks)
}

func TestIPBlocks_Apagar_AuditaEInexistenteNaoAcha(t *testing.T) {
	store, svc := servicoDeBloqueios()
	_, err := svc.Create(context.Background(), pedidoDoAdministrador())
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), "admin", "b1"))
	err = svc.Delete(context.Background(), "admin", "b1")

	require.ErrorIs(t, err, instance.ErrIPBlockNotFound)
	assert.Equal(t, "ip_blocks.delete", store.events[1].Action)
}

type relogioDeTeste struct{ agora time.Time }

func (r *relogioDeTeste) now() time.Time { return r.agora }

func listaRelida(store *fakeIPBlockStore) (*instance.RefreshingIPBlocks, *relogioDeTeste) {
	clock := &relogioDeTeste{agora: instanteDoBloqueio}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return instance.NewRefreshingIPBlocks(store, 5*time.Second, clock.now, logger), clock
}

func TestRefreshingIPBlocks_BloqueiaQuemCaiNaFaixa(t *testing.T) {
	store := &fakeIPBlockStore{blocks: []instance.IPBlock{{ID: "b1", Prefix: netip.MustParsePrefix("198.51.100.0/24")}}}
	lista, _ := listaRelida(store)

	assert.True(t, lista.Blocked(netip.MustParseAddr("198.51.100.9")))
	assert.False(t, lista.Blocked(netip.MustParseAddr("203.0.113.7")))
}

func TestRefreshingIPBlocks_ReleSoDepoisDoIntervalo(t *testing.T) {
	store := &fakeIPBlockStore{}
	lista, clock := listaRelida(store)
	lista.Blocked(netip.MustParseAddr("198.51.100.9"))
	store.blocks = []instance.IPBlock{{ID: "b1", Prefix: netip.MustParsePrefix("198.51.100.0/24")}}

	antes := lista.Blocked(netip.MustParseAddr("198.51.100.9"))
	clock.agora = clock.agora.Add(5 * time.Second)
	depois := lista.Blocked(netip.MustParseAddr("198.51.100.9"))

	assert.False(t, antes, "dentro do intervalo vale a lista lida")
	assert.True(t, depois)
	assert.Equal(t, 2, store.reads)
}

func TestRefreshingIPBlocks_BancoFora_MantemAUltimaLista(t *testing.T) {
	store := &fakeIPBlockStore{blocks: []instance.IPBlock{{ID: "b1", Prefix: netip.MustParsePrefix("198.51.100.0/24")}}}
	lista, clock := listaRelida(store)
	lista.Blocked(netip.MustParseAddr("198.51.100.9"))
	store.listErr = errors.New("banco fora")
	clock.agora = clock.agora.Add(time.Minute)

	assert.True(t, lista.Blocked(netip.MustParseAddr("198.51.100.9")))
}
