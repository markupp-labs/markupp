package cimd_test

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth/cimd"
)

const caminhoDoDocumento = "/oauth/client.json"

// permitirQualquerIP substitui a política padrão: o servidor de teste roda em 127.0.0.1.
func permitirQualquerIP(netip.Addr) error { return nil }

// servidorDeMetadados é o host remoto falso que publica o documento do cliente.
type servidorDeMetadados struct {
	*httptest.Server
	t            *testing.T
	documento    documentoDeTeste
	contentType  string
	cacheControl string
	status       int
	atraso       time.Duration
	substituto   http.Handler
	requisicoes  atomic.Int32
}

func novoServidorDeMetadados(t *testing.T) *servidorDeMetadados {
	t.Helper()
	srv := &servidorDeMetadados{t: t, contentType: "application/json", status: http.StatusOK}
	srv.Server = httptest.NewTLSServer(http.HandlerFunc(srv.servir))
	srv.documento = documentoValido(srv.clientID())
	t.Cleanup(srv.Close)
	return srv
}

func (s *servidorDeMetadados) clientID() string { return s.URL + caminhoDoDocumento }

func (s *servidorDeMetadados) servir(w http.ResponseWriter, r *http.Request) {
	s.requisicoes.Add(1)
	if s.substituto != nil {
		s.substituto.ServeHTTP(w, r)
		return
	}
	if s.atraso > 0 {
		select {
		case <-time.After(s.atraso):
		case <-r.Context().Done():
			return
		}
	}
	w.Header().Set("Content-Type", s.contentType)
	if s.cacheControl != "" {
		w.Header().Set("Cache-Control", s.cacheControl)
	}
	w.WriteHeader(s.status)
	_ = json.NewEncoder(w).Encode(s.documento)
}

func (s *servidorDeMetadados) confiaveis() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(s.Certificate())
	return pool
}

func (s *servidorDeMetadados) fetcher(opts ...cimd.Option) *cimd.Fetcher {
	base := []cimd.Option{cimd.WithRootCAs(s.confiaveis()), cimd.WithIPPolicy(permitirQualquerIP)}
	fetcher, err := cimd.NewFetcher(append(base, opts...)...)
	require.NoError(s.t, err)
	return fetcher
}

func (s *servidorDeMetadados) buscar(opts ...cimd.Option) (cimd.ClientMetadata, error) {
	return s.fetcher(opts...).Fetch(context.Background(), s.clientID())
}

// documentoDeTeste espelha o JSON publicado; campo nil fica fora do corpo, para os casos de ausência.
type documentoDeTeste struct {
	ClientID                *string   `json:"client_id,omitempty"`
	ClientName              *string   `json:"client_name,omitempty"`
	RedirectURIs            *[]string `json:"redirect_uris,omitempty"`
	TokenEndpointAuthMethod *string   `json:"token_endpoint_auth_method,omitempty"`
}

func documentoValido(clientID string) documentoDeTeste {
	return documentoDeTeste{
		ClientID:     new(clientID),
		ClientName:   new("Agente de Teste"),
		RedirectURIs: new([]string{"http://127.0.0.1/callback", "https://agente.example.com/cb"}),
	}
}

func TestFetcher_DocumentoValido_DevolveMetadadosComTTLPadrao(t *testing.T) {
	srv := novoServidorDeMetadados(t)
	srv.contentType = "application/json; charset=utf-8"

	meta, err := srv.buscar()

	require.NoError(t, err)
	assert.Equal(t, srv.clientID(), meta.ClientID)
	assert.Equal(t, "Agente de Teste", meta.Name)
	assert.Equal(t, []string{"http://127.0.0.1/callback", "https://agente.example.com/cb"}, meta.RedirectURIs)
	assert.Equal(t, time.Hour, meta.TTL)
}

func TestFetcher_AuthMethodNone_Aceita(t *testing.T) {
	srv := novoServidorDeMetadados(t)
	srv.documento.TokenEndpointAuthMethod = new("none")

	_, err := srv.buscar()

	require.NoError(t, err)
}

func TestFetcher_PoliticaPadrao_RecusaLoopbackSemEnviarRequisicao(t *testing.T) {
	srv := novoServidorDeMetadados(t)
	fetcher, err := cimd.NewFetcher(cimd.WithRootCAs(srv.confiaveis()))
	require.NoError(t, err)

	_, err = fetcher.Fetch(context.Background(), srv.clientID())

	require.ErrorIs(t, err, cimd.ErrBlockedAddress)
	assert.Zero(t, srv.requisicoes.Load())
}

func TestFetcher_CertificadoNaoConfiavel_Recusa(t *testing.T) {
	srv := novoServidorDeMetadados(t)
	fetcher, err := cimd.NewFetcher(cimd.WithIPPolicy(permitirQualquerIP))
	require.NoError(t, err)

	_, err = fetcher.Fetch(context.Background(), srv.clientID())

	require.ErrorIs(t, err, cimd.ErrFetchFailed)
}

func TestFetcher_ClientIDInvalido_RecusaSemBuscar(t *testing.T) {
	srv := novoServidorDeMetadados(t)
	inseguro := strings.Replace(srv.clientID(), "https://", "http://", 1)

	_, err := srv.fetcher().Fetch(context.Background(), inseguro)

	require.ErrorIs(t, err, cimd.ErrInvalidClientID)
	assert.Zero(t, srv.requisicoes.Load())
}

func TestFetcher_RespostaRecusada_DevolveErroDeBusca(t *testing.T) {
	casos := map[string]func(*servidorDeMetadados){
		"redirecionamento":   func(s *servidorDeMetadados) { s.status = http.StatusFound },
		"nao encontrado":     func(s *servidorDeMetadados) { s.status = http.StatusNotFound },
		"content-type html":  func(s *servidorDeMetadados) { s.contentType = "text/html" },
		"content-type vazio": func(s *servidorDeMetadados) { s.contentType = "" },
	}
	for nome, preparar := range casos {
		t.Run(nome, func(t *testing.T) {
			srv := novoServidorDeMetadados(t)
			preparar(srv)

			_, err := srv.buscar()

			require.ErrorIs(t, err, cimd.ErrFetchFailed)
		})
	}
}

func TestFetcher_RedirecionamentoComLocation_NaoSegue(t *testing.T) {
	srv := novoServidorDeMetadados(t)
	srv.substituto = http.RedirectHandler(srv.URL+"/outro", http.StatusMovedPermanently)

	_, err := srv.buscar()

	require.ErrorIs(t, err, cimd.ErrFetchFailed)
	assert.Contains(t, err.Error(), "301")
}

func TestFetcher_CorpoAcimaDoLimite_Recusa(t *testing.T) {
	srv := novoServidorDeMetadados(t)

	_, err := srv.buscar(cimd.WithMaxBodyBytes(32))

	require.ErrorIs(t, err, cimd.ErrFetchFailed)
	assert.Contains(t, err.Error(), "32")
}

func TestFetcher_ServidorLento_EstouraTimeout(t *testing.T) {
	srv := novoServidorDeMetadados(t)
	srv.atraso = time.Second

	inicio := time.Now()
	_, err := srv.buscar(cimd.WithTimeout(50 * time.Millisecond))

	require.ErrorIs(t, err, cimd.ErrFetchFailed)
	assert.Less(t, time.Since(inicio), 500*time.Millisecond)
}

func TestFetcher_DocumentoInvalido_Recusa(t *testing.T) {
	casos := map[string]func(d *documentoDeTeste){
		"client_id diferente":        func(d *documentoDeTeste) { d.ClientID = new("https://outro.example.com/c.json") },
		"client_id ausente":          func(d *documentoDeTeste) { d.ClientID = nil },
		"redirect_uris ausente":      func(d *documentoDeTeste) { d.RedirectURIs = nil },
		"redirect_uris vazio":        func(d *documentoDeTeste) { d.RedirectURIs = new([]string{}) },
		"redirect_uri relativa":      func(d *documentoDeTeste) { d.RedirectURIs = new([]string{"/callback"}) },
		"redirect_uri com fragmento": func(d *documentoDeTeste) { d.RedirectURIs = new([]string{"https://a.example.com/cb#x"}) },
		"client_name ausente":        func(d *documentoDeTeste) { d.ClientName = nil },
		"client_name em branco":      func(d *documentoDeTeste) { d.ClientName = new("   ") },
		"auth method com segredo":    func(d *documentoDeTeste) { d.TokenEndpointAuthMethod = new("client_secret_basic") },
		"auth method vazio":          func(d *documentoDeTeste) { d.TokenEndpointAuthMethod = new("") },
	}
	for nome, alterar := range casos {
		t.Run(nome, func(t *testing.T) {
			srv := novoServidorDeMetadados(t)
			alterar(&srv.documento)

			_, err := srv.buscar()

			require.ErrorIs(t, err, cimd.ErrInvalidDocument)
		})
	}
}

func TestFetcher_JSONMalformado_Recusa(t *testing.T) {
	srv := novoServidorDeMetadados(t)
	srv.substituto = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"client_id": `))
	})

	_, err := srv.buscar()

	require.ErrorIs(t, err, cimd.ErrInvalidDocument)
}

func TestFetcher_CacheControl_DefineTTLDentroDosLimites(t *testing.T) {
	casos := map[string]time.Duration{
		"":                          time.Hour,
		"public":                    time.Hour,
		"max-age=600":               10 * time.Minute,
		"public, max-age=7200":      2 * time.Hour,
		"max-age=60":                5 * time.Minute,
		"max-age=0":                 5 * time.Minute,
		"max-age=172800":            24 * time.Hour,
		"max-age=99999999999999999": 24 * time.Hour,
		"max-age=abc":               5 * time.Minute,
		"no-store":                  5 * time.Minute,
		"no-cache, max-age=7200":    5 * time.Minute,
		"Max-Age=600":               10 * time.Minute,
	}
	for cabecalho, esperado := range casos {
		t.Run(cabecalho, func(t *testing.T) {
			srv := novoServidorDeMetadados(t)
			srv.cacheControl = cabecalho

			meta, err := srv.buscar()

			require.NoError(t, err)
			assert.Equal(t, esperado, meta.TTL)
		})
	}
}

func TestFetcher_LimitesDeTTLConfigurados_Respeita(t *testing.T) {
	casos := map[string]time.Duration{"max-age=30": time.Minute, "max-age=99999": 2 * time.Hour, "": time.Hour}
	for cabecalho, esperado := range casos {
		t.Run(cabecalho, func(t *testing.T) {
			srv := novoServidorDeMetadados(t)
			srv.cacheControl = cabecalho

			meta, err := srv.buscar(cimd.WithTTLBounds(time.Minute, 2*time.Hour))

			require.NoError(t, err)
			assert.Equal(t, esperado, meta.TTL)
		})
	}
}
