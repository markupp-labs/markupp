package cimd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"time"
)

// ErrInvalidOption indica uma opção do construtor fora da faixa aceita.
var ErrInvalidOption = errors.New("opcao do fetcher invalida")

// ErrFetchFailed indica que a busca do documento falhou antes de chegar ao JSON.
var ErrFetchFailed = errors.New("busca do documento de metadados falhou")

// Padrões do Fetcher, ajustáveis pelas opções do construtor.
const (
	DefaultTimeout      = 5 * time.Second
	DefaultMaxBodyBytes = 64 << 10
	DefaultMinTTL       = 5 * time.Minute
	DefaultMaxTTL       = 24 * time.Hour
)

// Fetcher busca e valida Client ID Metadata Documents.
type Fetcher struct {
	client  *http.Client
	policy  IPPolicy
	rootCAs *x509.CertPool
	timeout time.Duration
	maxBody int64
	minTTL  time.Duration
	maxTTL  time.Duration
}

// Option ajusta um Fetcher em construção.
type Option func(*Fetcher)

// WithTimeout limita o tempo total da busca.
func WithTimeout(timeout time.Duration) Option { return func(f *Fetcher) { f.timeout = timeout } }

// WithMaxBodyBytes limita o tamanho do corpo; acima disso a busca falha.
func WithMaxBodyBytes(limit int64) Option { return func(f *Fetcher) { f.maxBody = limit } }

// WithTTLBounds define o intervalo em que o TTL de cache é limitado.
func WithTTLBounds(minTTL, maxTTL time.Duration) Option {
	return func(f *Fetcher) { f.minTTL, f.maxTTL = minTTL, maxTTL }
}

// WithIPPolicy troca a política de IP; o padrão é PublicOnly.
func WithIPPolicy(policy IPPolicy) Option { return func(f *Fetcher) { f.policy = policy } }

// WithRootCAs troca as autoridades confiáveis; o padrão (nil) usa as do sistema.
func WithRootCAs(pool *x509.CertPool) Option { return func(f *Fetcher) { f.rootCAs = pool } }

// NewFetcher monta o Fetcher com um cliente HTTP próprio, isolado do DefaultClient.
func NewFetcher(opts ...Option) (*Fetcher, error) {
	f := &Fetcher{
		policy:  PublicOnly,
		timeout: DefaultTimeout,
		maxBody: DefaultMaxBodyBytes,
		minTTL:  DefaultMinTTL,
		maxTTL:  DefaultMaxTTL,
	}
	for _, opt := range opts {
		opt(f)
	}
	if err := f.validateOptions(); err != nil {
		return nil, err
	}
	f.client = newGuardedClient(f.policy, f.rootCAs, f.timeout)
	return f, nil
}

// validateOptions recusa valores que deixariam a busca sem limite de tempo, o corpo
// sempre vazio (ou limit+1 estourando em readLimited), o TTL fora de um intervalo ou
// a conexão sem política de IP (o Control entraria em pânico no dial).
func (f *Fetcher) validateOptions() error {
	if f.policy == nil {
		return fmt.Errorf("%w: politica de IP nil; esperado IPPolicy nao nula", ErrInvalidOption)
	}
	if f.timeout <= 0 {
		return fmt.Errorf("%w: timeout %s; esperado maior que zero", ErrInvalidOption, f.timeout)
	}
	if f.maxBody <= 0 || f.maxBody == math.MaxInt64 {
		return fmt.Errorf("%w: limite de corpo %d bytes; esperado entre 1 e %d",
			ErrInvalidOption, f.maxBody, int64(math.MaxInt64-1))
	}
	if f.minTTL <= 0 || f.minTTL > f.maxTTL {
		return fmt.Errorf("%w: TTL minimo %s e maximo %s; esperado 0 < minimo <= maximo",
			ErrInvalidOption, f.minTTL, f.maxTTL)
	}
	return nil
}

// newGuardedClient não usa proxy (o proxy esconderia o IP real do Control) e não segue
// redirecionamentos: o 3xx volta como resposta e vira erro em checkResponse.
func newGuardedClient(policy IPPolicy, roots *x509.CertPool, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, Control: policy.dialControl}
	transport := &http.Transport{
		DialContext:       dialer.DialContext,
		TLSClientConfig:   &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2: true,
	}
	return &http.Client{
		Transport:     transport,
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// Fetch valida o client_id, busca o documento e devolve os metadados com o TTL de cache.
func (f *Fetcher) Fetch(ctx context.Context, clientID string) (ClientMetadata, error) {
	if _, err := ParseClientID(clientID); err != nil {
		return ClientMetadata{}, err
	}
	body, cacheControl, err := f.download(ctx, clientID)
	if err != nil {
		return ClientMetadata{}, err
	}
	meta, err := parseDocument(body, clientID)
	if err != nil {
		return ClientMetadata{}, err
	}
	meta.TTL = cacheTTL(cacheControl, f.minTTL, f.maxTTL)
	return meta, nil
}

func (f *Fetcher) download(ctx context.Context, target string) (body []byte, cacheControl string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return nil, "", fmt.Errorf("%w: GET %s: %w", ErrFetchFailed, target, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("%w: GET %s: %w", ErrFetchFailed, target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := checkResponse(resp); err != nil {
		return nil, "", err
	}
	body, err = readLimited(resp.Body, f.maxBody)
	return body, resp.Header.Get("Cache-Control"), err
}

func checkResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d em %s; esperado 200 sem redirecionamento",
			ErrFetchFailed, resp.StatusCode, resp.Request.URL)
	}
	contentType := resp.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return fmt.Errorf("%w: Content-Type %q; esperado application/json", ErrFetchFailed, contentType)
	}
	return nil
}

func readLimited(body io.Reader, limit int64) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: leitura do corpo: %w", ErrFetchFailed, err)
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("%w: corpo com mais de %d bytes; esperado ate %d", ErrFetchFailed, limit, limit)
	}
	return content, nil
}
