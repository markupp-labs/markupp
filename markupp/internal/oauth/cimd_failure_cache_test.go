package oauth_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// testFailureTTL é o tempo do cache negativo do CIMD nos testes.
const testFailureTTL = 5 * time.Minute

// manualClock é o relógio que o teste avança à mão.
type manualClock struct{ now time.Time }

func (c *manualClock) read() time.Time { return c.now }

// newFailureCacheService é o Service com cache negativo de testFailureTTL e o relógio dado.
func newFailureCacheService(repo *fakeOAuthRepository, fetcher *fakeClientMetadataFetcher, clock *manualClock) *oauth.Service {
	cfg := testServiceConfig()
	cfg.CIMDFailureTTL = testFailureTTL
	return newCIMDTestService(repo, fetcher, cfg, clock.read)
}

func TestStartAuthorization_CIMDFalhaDeRedeRecente_RecusaIgualSemBuscarDeNovo(t *testing.T) {
	repo, clock := newFakeOAuthRepository(), &manualClock{now: testNow}
	fetcher := &fakeClientMetadataFetcher{err: errors.New("dial tcp 10.0.0.7:443: connection refused")}
	svc := newFailureCacheService(repo, fetcher, clock)
	_, primeira := svc.StartAuthorization(context.Background(), validAuthorizeParams())
	requireProtocolError(t, primeira, "invalid_request", "", "")

	clock.now = testNow.Add(testFailureTTL - time.Second)
	_, err := svc.StartAuthorization(context.Background(), validAuthorizeParams())

	requireProtocolError(t, err, "invalid_request", "", "")
	assert.Equal(t, primeira.Error(), err.Error(), "a mesma recusa genérica")
	assert.Equal(t, []string{testClientID}, fetcher.calls)
	assert.Empty(t, repo.savedCIMD)
	assert.Empty(t, repo.created)
}

// fetchersSemRede falham sem que a busca tenha passado pela rede: o client_id recusado
// pelo fetcher e o documento recusado pelas guardas do serviço.
func fetchersSemRede() map[string]func() *fakeClientMetadataFetcher {
	return map[string]func() *fakeClientMetadataFetcher{
		"client_id invalido": func() *fakeClientMetadataFetcher {
			return &fakeClientMetadataFetcher{err: fmt.Errorf("buscar: %w: com porta 8443", oauth.ErrInvalidMetadataClientID)}
		},
		"documento sem ttl": func() *fakeClientMetadataFetcher {
			fetcher := testMetadataFetcher()
			fetcher.metadata.TTL = 0
			return fetcher
		},
		"documento de outro client_id": func() *fakeClientMetadataFetcher {
			fetcher := testMetadataFetcher()
			fetcher.metadata.ClientID = "https://outro.exemplo.dev/oauth.json"
			return fetcher
		},
	}
}

func TestStartAuthorization_CIMDFalhaSemRede_RecusaSemLembrar(t *testing.T) {
	for nome, novoFetcher := range fetchersSemRede() {
		t.Run(nome, func(t *testing.T) {
			repo, fetcher, clock := newFakeOAuthRepository(), novoFetcher(), &manualClock{now: testNow}
			svc := newFailureCacheService(repo, fetcher, clock)
			_, primeira := svc.StartAuthorization(context.Background(), validAuthorizeParams())
			requireProtocolError(t, primeira, "invalid_request", "", "")

			_, err := svc.StartAuthorization(context.Background(), validAuthorizeParams())

			requireProtocolError(t, err, "invalid_request", "", "")
			assert.Equal(t, []string{testClientID, testClientID}, fetcher.calls, "sem cache negativo, a busca acontece de novo")
			assert.Empty(t, repo.savedCIMD)
		})
	}
}

func TestStartAuthorization_CIMDFalhaVencida_BuscaDeNovo(t *testing.T) {
	repo, fetcher, clock := newFakeOAuthRepository(), testMetadataFetcher(), &manualClock{now: testNow}
	fetcher.err = errors.New("dial tcp 10.0.0.7:443: connection refused")
	svc := newFailureCacheService(repo, fetcher, clock)
	_, err := svc.StartAuthorization(context.Background(), validAuthorizeParams())
	require.Error(t, err)

	fetcher.err = nil
	clock.now = testNow.Add(testFailureTTL)
	_, err = svc.StartAuthorization(context.Background(), validAuthorizeParams())

	require.NoError(t, err)
	assert.Equal(t, []string{testClientID, testClientID}, fetcher.calls)
	assert.Len(t, repo.created, 1)
}

func TestStartAuthorization_CIMDFalhaDeOutroCliente_NaoAfeta(t *testing.T) {
	repo, fetcher, clock := newFakeOAuthRepository(), testMetadataFetcher(), &manualClock{now: testNow}
	fetcher.err = errors.New("dial tcp 10.0.0.7:443: connection refused")
	svc := newFailureCacheService(repo, fetcher, clock)
	outro := validAuthorizeParams()
	outro.ClientID = "https://outro.exemplo.dev/oauth.json"
	_, err := svc.StartAuthorization(context.Background(), outro)
	require.Error(t, err)

	fetcher.err = nil
	_, err = svc.StartAuthorization(context.Background(), validAuthorizeParams())

	require.NoError(t, err)
	assert.Equal(t, []string{outro.ClientID, testClientID}, fetcher.calls)
}

func TestStartAuthorization_CIMDPedidoCancelado_BuscaSemOCancelamento(t *testing.T) {
	repo, fetcher, clock := newFakeOAuthRepository(), testMetadataFetcher(), &manualClock{now: testNow}
	svc := newFailureCacheService(repo, fetcher, clock)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.StartAuthorization(ctx, validAuthorizeParams())

	require.NoError(t, err)
	assert.Equal(t, []error{nil}, fetcher.ctxErrs, "quem derruba a conexão não envenena o cache negativo")
}

func TestStartAuthorization_CIMDClientIDLongoDemais_RecusaSemBuscar(t *testing.T) {
	repo, fetcher, clock := newFakeOAuthRepository(), testMetadataFetcher(), &manualClock{now: testNow}
	svc := newFailureCacheService(repo, fetcher, clock)
	params := validAuthorizeParams()
	params.ClientID = "https://cliente.exemplo.dev/" + strings.Repeat("a", 2048)

	_, err := svc.StartAuthorization(context.Background(), params)

	requireProtocolError(t, err, "invalid_request", "", "")
	assert.Empty(t, fetcher.calls)
}
