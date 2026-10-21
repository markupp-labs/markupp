package oauth_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// maxBuscasCIMD é o limite de buscas de documento CIMD em andamento por réplica.
const maxBuscasCIMD = 8

// esperaDasBuscas é quanto o teste espera por uma busca que deveria começar.
const esperaDasBuscas = 5 * time.Second

// lockedOAuthRepository serializa o fakeOAuthRepository para pedidos simultâneos.
type lockedOAuthRepository struct {
	*fakeOAuthRepository
	mu sync.Mutex
}

func newLockedOAuthRepository() *lockedOAuthRepository {
	return &lockedOAuthRepository{fakeOAuthRepository: newFakeOAuthRepository()}
}

func (r *lockedOAuthRepository) GetFreshCIMDClient(ctx context.Context, id string, now time.Time) (oauth.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeOAuthRepository.GetFreshCIMDClient(ctx, id, now)
}

func (r *lockedOAuthRepository) SaveCIMDClient(ctx context.Context, client oauth.Client, expiresAt, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeOAuthRepository.SaveCIMDClient(ctx, client, expiresAt, now)
}

func (r *lockedOAuthRepository) CreateRequest(ctx context.Context, req oauth.AuthorizationRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeOAuthRepository.CreateRequest(ctx, req)
}

// lookups conta as leituras do cache, feitas logo antes da busca.
func (r *lockedOAuthRepository) lookups() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.freshCIMDCalls)
}

// blockingMetadataFetcher segura cada busca até release fechar, avisa em started cada
// client_id buscado e devolve um documento válido para ele. Sem release a tempo, a busca
// falha, para o teste vermelho falhar em vez de travar.
type blockingMetadataFetcher struct {
	started chan string
	release chan struct{}
	mu      sync.Mutex
	calls   []string
}

func newBlockingMetadataFetcher() *blockingMetadataFetcher {
	return &blockingMetadataFetcher{started: make(chan string, 64), release: make(chan struct{})}
}

func (f *blockingMetadataFetcher) FetchClientMetadata(_ context.Context, clientID string) (oauth.ClientMetadata, error) {
	f.mu.Lock()
	f.calls = append(f.calls, clientID)
	f.mu.Unlock()
	f.started <- clientID
	select {
	case <-f.release:
	case <-time.After(esperaDasBuscas):
		return oauth.ClientMetadata{}, fmt.Errorf("busca de %s segurada por mais de %s; esperado release antes", clientID, esperaDasBuscas)
	}
	return oauth.ClientMetadata{ClientID: clientID, Name: "Agente", RedirectURIs: testClient().RedirectURIs, TTL: testMetadataTTL}, nil
}

func (f *blockingMetadataFetcher) fetched() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// waitStarted espera n buscas começarem e falha o teste se elas não vierem a tempo.
func (f *blockingMetadataFetcher) waitStarted(t *testing.T, n int) {
	t.Helper()
	for range n {
		select {
		case <-f.started:
		case <-time.After(esperaDasBuscas):
			require.FailNow(t, "busca de documento cimd esperada não começou")
		}
	}
}

// newBlockingCIMDService monta o Service sobre o repositório serializado, com cache
// negativo ligado para provar que a recusa por falta de lugar não entra nele.
func newBlockingCIMDService(repo *lockedOAuthRepository, fetcher *blockingMetadataFetcher) *oauth.Service {
	cfg := testServiceConfig()
	cfg.CIMDFailureTTL = testFailureTTL
	ids := &lockedSequence{source: sequenceSource{prefix: "pedido"}}
	tokens := &lockedSequence{source: sequenceSource{prefix: "token"}}
	return oauth.NewService(repo, cfg, nil, fetcher, func() time.Time { return testNow }, ids.next, tokens.next)
}

// lockedSequence serializa o sequenceSource para pedidos simultâneos.
type lockedSequence struct {
	mu     sync.Mutex
	source sequenceSource
}

func (l *lockedSequence) next() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.source.next()
}

func authorizeParamsFor(clientID string) oauth.AuthorizeParams {
	params := validAuthorizeParams()
	params.ClientID = clientID
	return params
}

// startAuthorizations dispara um StartAuthorization por client_id e devolve o canal com
// o erro de cada um.
func startAuthorizations(svc *oauth.Service, clientIDs []string) <-chan error {
	errs := make(chan error, len(clientIDs))
	for _, clientID := range clientIDs {
		go func() {
			_, err := svc.StartAuthorization(context.Background(), authorizeParamsFor(clientID))
			errs <- err
		}()
	}
	return errs
}

func requireAllSucceeded(t *testing.T, errs <-chan error, n int) {
	t.Helper()
	for range n {
		require.NoError(t, <-errs)
	}
}

func TestStartAuthorization_CIMDBuscasSimultaneasDoMesmoCliente_ViramUma(t *testing.T) {
	repo, fetcher := newLockedOAuthRepository(), newBlockingMetadataFetcher()
	svc := newBlockingCIMDService(repo, fetcher)
	clientIDs := []string{testClientID, testClientID, testClientID, testClientID, testClientID}

	errs := startAuthorizations(svc, clientIDs)
	fetcher.waitStarted(t, 1)
	require.Eventually(t, func() bool { return repo.lookups() == len(clientIDs) }, esperaDasBuscas, time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	close(fetcher.release)

	requireAllSucceeded(t, errs, len(clientIDs))
	assert.Equal(t, []string{testClientID}, fetcher.fetched(), "as buscas simultâneas do mesmo client_id viram uma")
}

func distinctCIMDClientIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("https://cliente-%d.exemplo.dev/oauth.json", i)
	}
	return ids
}

// genericRefusal é a recusa que o client_id recebe quando a busca do documento falha.
func genericRefusal(t *testing.T, clientID string) string {
	t.Helper()
	failing := &fakeClientMetadataFetcher{err: errors.New("dial tcp 10.0.0.7:443: connection refused")}
	_, err := newTestServiceWithMetadata(newFakeOAuthRepository(), failing).StartAuthorization(context.Background(), authorizeParamsFor(clientID))
	require.Error(t, err)
	return err.Error()
}

func TestStartAuthorization_CIMDBuscasNoLimite_RecusaANovaSemBuscar(t *testing.T) {
	repo, fetcher := newLockedOAuthRepository(), newBlockingMetadataFetcher()
	svc := newBlockingCIMDService(repo, fetcher)
	errs := startAuthorizations(svc, distinctCIMDClientIDs(maxBuscasCIMD))
	fetcher.waitStarted(t, maxBuscasCIMD)
	excedente := "https://excedente.exemplo.dev/oauth.json"

	_, err := svc.StartAuthorization(context.Background(), authorizeParamsFor(excedente))

	requireProtocolError(t, err, "invalid_request", "", "")
	assert.Equal(t, genericRefusal(t, excedente), err.Error(), "a mesma recusa genérica")
	assert.NotContains(t, fetcher.fetched(), excedente, "acima do limite não busca")
	close(fetcher.release)
	requireAllSucceeded(t, errs, maxBuscasCIMD)
	_, err = svc.StartAuthorization(context.Background(), authorizeParamsFor(excedente))
	require.NoError(t, err, "com lugar livre a busca volta, sem cache negativo da recusa")
}
