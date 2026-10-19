package oauth_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// testMetadataTTL é o tempo de cache do documento devolvido pelo fetcher de teste.
const testMetadataTTL = 30 * time.Minute

// newTestServiceWithMetadata é o newTestService com o fetcher de documentos CIMD.
func newTestServiceWithMetadata(repo *fakeOAuthRepository, metadata oauth.ClientMetadataFetcher) *oauth.Service {
	return newCIMDTestService(repo, metadata, testServiceConfig(), func() time.Time { return testNow })
}

// newCIMDTestService monta o Service com o fetcher, a config e o relógio dados.
func newCIMDTestService(repo *fakeOAuthRepository, metadata oauth.ClientMetadataFetcher, cfg oauth.ServiceConfig, clock func() time.Time) *oauth.Service {
	ids := &sequenceSource{prefix: "pedido"}
	tokens := &sequenceSource{prefix: "token"}
	return oauth.NewService(repo, cfg, nil, metadata, clock, ids.next, tokens.next)
}

func testMetadataFetcher() *fakeClientMetadataFetcher {
	client := testClient()
	return &fakeClientMetadataFetcher{metadata: oauth.ClientMetadata{
		ClientID: client.ID, Name: client.Name, RedirectURIs: client.RedirectURIs, TTL: testMetadataTTL,
	}}
}

func TestStartAuthorization_CIMDEmCacheFresco_NaoBusca(t *testing.T) {
	repo := newFakeOAuthRepository()
	repo.freshCIMD[testClientID] = testClient()
	fetcher := testMetadataFetcher()

	_, err := newTestServiceWithMetadata(repo, fetcher).StartAuthorization(context.Background(), validAuthorizeParams())

	require.NoError(t, err)
	assert.Equal(t, []freshCIMDLookup{{ID: testClientID, Now: testNow}}, repo.freshCIMDCalls)
	assert.Empty(t, fetcher.calls)
	assert.Empty(t, repo.savedCIMD)
	assert.Empty(t, repo.getClientCalls)
	assert.Len(t, repo.created, 1)
}

func TestStartAuthorization_CIMDSemCache_BuscaEGravaComExpiracao(t *testing.T) {
	repo := newFakeOAuthRepository()
	fetcher := testMetadataFetcher()

	_, err := newTestServiceWithMetadata(repo, fetcher).StartAuthorization(context.Background(), validAuthorizeParams())

	require.NoError(t, err)
	assert.Equal(t, []string{testClientID}, fetcher.calls)
	assert.Equal(t, []savedCIMDClient{{Client: testClient(), ExpiresAt: testNow.Add(testMetadataTTL)}}, repo.savedCIMD)
	assert.Empty(t, repo.getClientCalls)
	assert.Len(t, repo.created, 1)
}

// O caso do documento vencido deixa o cliente gravado, que GetClient acharia, para
// garantir que a cópia vencida não é usada quando a nova busca falha.
func TestStartAuthorization_CIMDBuscaFalha_RecusaSemRedirectNemDetalhe(t *testing.T) {
	casos := map[string]func(*fakeOAuthRepository){
		"sem cache":         func(*fakeOAuthRepository) {},
		"documento vencido": func(r *fakeOAuthRepository) { r.clients[testClientID] = testClient() },
	}
	for nome, prepara := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := newFakeOAuthRepository()
			prepara(repo)
			fetcher := &fakeClientMetadataFetcher{err: errors.New("dial tcp 10.0.0.7:443: connection refused")}

			_, err := newTestServiceWithMetadata(repo, fetcher).StartAuthorization(context.Background(), validAuthorizeParams())

			requireProtocolError(t, err, "invalid_request", "", "")
			assert.ErrorIs(t, err, fetcher.err, "a causa segue para o log")
			assert.NotContains(t, err.Error(), "10.0.0.7")
			assert.NotContains(t, err.Error(), "connection refused")
			assert.Equal(t, []string{testClientID}, fetcher.calls, "sem documento fresco, a busca acontece")
			assert.Empty(t, repo.savedCIMD)
			assert.Empty(t, repo.created)
		})
	}
}

func TestStartAuthorization_CIMDFalhaDoRepositorio_DevolveErroEmbrulhado(t *testing.T) {
	casos := map[string]func(*fakeOAuthRepository) error{
		"lendo o cache": func(r *fakeOAuthRepository) error {
			r.getFreshCIMDErr = errors.New("banco fora do ar")
			return r.getFreshCIMDErr
		},
		"gravando o documento": func(r *fakeOAuthRepository) error {
			r.saveCIMDErr = errors.New("banco fora do ar")
			return r.saveCIMDErr
		},
	}
	for nome, quebra := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := newFakeOAuthRepository()
			cause := quebra(repo)

			_, err := newTestServiceWithMetadata(repo, testMetadataFetcher()).StartAuthorization(context.Background(), validAuthorizeParams())

			requireNotProtocolError(t, err, cause)
			assert.Empty(t, repo.created)
		})
	}
}

func TestStartAuthorization_ClientIDQueNaoEURLHTTPS_UsaGetClient(t *testing.T) {
	casos := map[string]string{
		"identificador opaco": "cliente-dcr-1",
		"url http":            "http://cliente.exemplo.dev/oauth.json",
	}
	for nome, clientID := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := newFakeOAuthRepository()
			repo.clients[clientID] = oauth.Client{ID: clientID, RedirectURIs: []string{testRedirectURI}}
			fetcher := testMetadataFetcher()
			params := validAuthorizeParams()
			params.ClientID = clientID

			_, err := newTestServiceWithMetadata(repo, fetcher).StartAuthorization(context.Background(), params)

			require.NoError(t, err)
			assert.Equal(t, []string{clientID}, repo.getClientCalls)
			assert.Empty(t, repo.freshCIMDCalls)
			assert.Empty(t, fetcher.calls)
		})
	}
}

func TestStartAuthorization_SemFetcherComClientIDURL_UsaGetClient(t *testing.T) {
	repo := testRepositoryWithClient()

	_, err := newTestServiceWithMetadata(repo, nil).StartAuthorization(context.Background(), validAuthorizeParams())

	require.NoError(t, err)
	assert.Equal(t, []string{testClientID}, repo.getClientCalls)
	assert.Empty(t, repo.freshCIMDCalls)
}

func TestStartAuthorization_CIMDBuscadoSemARedirect_RecusaSemRedirect(t *testing.T) {
	repo := newFakeOAuthRepository()
	params := validAuthorizeParams()
	params.RedirectURI = "https://cliente.exemplo.dev/outra"
	fetcher := testMetadataFetcher()

	_, err := newTestServiceWithMetadata(repo, fetcher).StartAuthorization(context.Background(), params)

	requireProtocolError(t, err, "invalid_request", "", "")
	assert.Equal(t, []string{testClientID}, fetcher.calls)
	assert.Equal(t, []savedCIMDClient{{Client: testClient(), ExpiresAt: testNow.Add(testMetadataTTL)}}, repo.savedCIMD,
		"o documento válido fica em cache mesmo sem a redirect_uri pedida")
	assert.Empty(t, repo.created)
}

func TestStartAuthorization_CIMDDocumentoForaDoContrato_RecusaSemGravar(t *testing.T) {
	casos := map[string]func(*oauth.ClientMetadata){
		"client_id diferente": func(m *oauth.ClientMetadata) { m.ClientID = "https://outro.exemplo.dev/oauth.json" },
		"sem redirect_uris":   func(m *oauth.ClientMetadata) { m.RedirectURIs = nil },
		"ttl zero":            func(m *oauth.ClientMetadata) { m.TTL = 0 },
		"ttl negativo":        func(m *oauth.ClientMetadata) { m.TTL = -time.Minute },
	}
	for nome, estraga := range casos {
		t.Run(nome, func(t *testing.T) {
			repo := newFakeOAuthRepository()
			fetcher := testMetadataFetcher()
			estraga(&fetcher.metadata)

			_, err := newTestServiceWithMetadata(repo, fetcher).StartAuthorization(context.Background(), validAuthorizeParams())

			requireProtocolError(t, err, "invalid_request", "", "")
			assert.Contains(t, err.Error(), "inacessivel ou invalido", "a mesma recusa genérica da busca falhando")
			assert.Empty(t, repo.savedCIMD)
			assert.Empty(t, repo.created)
		})
	}
}

func TestStartAuthorization_CIMDConflitoDeOrigem_RecusaComOInvalidRequestGenerico(t *testing.T) {
	repo := newFakeOAuthRepository()
	repo.saveCIMDErr = fmt.Errorf("salvar cliente cimd: %w", oauth.ErrClientSourceConflict)

	_, err := newTestServiceWithMetadata(repo, testMetadataFetcher()).StartAuthorization(context.Background(), validAuthorizeParams())

	requireProtocolError(t, err, "invalid_request", "", "")
	assert.Contains(t, err.Error(), "inacessivel ou invalido", "a mesma recusa genérica da busca falhando")
	assert.ErrorIs(t, err, oauth.ErrClientSourceConflict, "a causa segue para o log")
	assert.Empty(t, repo.created)
}
