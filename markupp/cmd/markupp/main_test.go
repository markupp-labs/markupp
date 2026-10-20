package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth/cimd"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer_PortaConfigurada_DefineEnderecoETimeouts(t *testing.T) {
	server := newServer(8080, http.NotFoundHandler())

	assert.Equal(t, ":8080", server.Addr)
	assert.Greater(t, server.ReadHeaderTimeout, time.Duration(0),
		"ReadHeaderTimeout zerado deixa o servidor exposto a slowloris")
	assert.Greater(t, server.ReadTimeout, time.Duration(0))
	assert.Greater(t, server.WriteTimeout, time.Duration(0))
	assert.Greater(t, server.IdleTimeout, time.Duration(0))
}

func TestParseCommand_SemArgumento_RetornaServe(t *testing.T) {
	cmd, err := parseCommand([]string{"markupp"})

	require.NoError(t, err)
	assert.Equal(t, serveCommand, cmd)
}

func TestParseCommand_Migrate_RetornaMigrate(t *testing.T) {
	cmd, err := parseCommand([]string{"markupp", "migrate"})

	require.NoError(t, err)
	assert.Equal(t, migrateCommand, cmd)
}

func TestParseCommand_Desconhecido_RetornaErroComOValor(t *testing.T) {
	_, err := parseCommand([]string{"markupp", "migrar"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "migrar")
	assert.Contains(t, err.Error(), "migrate")
}

func TestMigrateDatabase_BancoVazio_CriaTabelaNotes(t *testing.T) {
	url := storagetest.DatabaseURL(t)

	require.NoError(t, migrateDatabase(context.Background(), url, ""))

	pool, err := storage.OpenPool(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	var tabelaNotes, tabelaUsers *string
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT to_regclass('notes')::text").Scan(&tabelaNotes))
	assert.NotNil(t, tabelaNotes, "esperada a tabela notes depois do migrate")
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT to_regclass('users')::text").Scan(&tabelaUsers))
	assert.NotNil(t, tabelaUsers, "esperada a tabela users depois do migrate")
}

func TestMigrateDatabase_BancoInalcancavel_RetornaErro(t *testing.T) {
	err := migrateDatabase(context.Background(), "postgres://ninguem@127.0.0.1:1/nada?connect_timeout=1", "")

	assert.Error(t, err)
}

func TestParseCommand_Index_RetornaIndexacaoUnica(t *testing.T) {
	cmd, err := parseCommand([]string{"markupp", "index"})

	require.NoError(t, err)
	assert.Equal(t, indexCommand, cmd)
}

func TestParseCommand_IndexLoop_RetornaIndexacaoEmLaco(t *testing.T) {
	cmd, err := parseCommand([]string{"markupp", "index-loop"})

	require.NoError(t, err)
	assert.Equal(t, indexLoopCommand, cmd)
}

func TestParseCommand_Desconhecido_ListaTodosOsSubcomandos(t *testing.T) {
	_, err := parseCommand([]string{"markupp", "indexar"})

	require.Error(t, err)
	for _, esperado := range []string{"indexar", "serve", "migrate", "index", "index-loop"} {
		assert.Contains(t, err.Error(), esperado)
	}
}

func TestRun_Index_IndexaOCofrePendenteESai(t *testing.T) {
	url := storagetest.DatabaseURL(t)
	require.NoError(t, migrateDatabase(context.Background(), url, ""))
	pool, err := storage.OpenPool(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	ctx := storagetest.SeedMember(t, pool, identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}, "va")
	agora := time.Now()
	require.NoError(t, storage.NewPostgresNotesRepository(pool).Save(ctx,
		notes.Note{ID: "n1", VaultID: "va", Path: "a.md", Content: "oi", CreatedAt: agora, UpdatedAt: agora}))
	t.Setenv("MARKUPP_DATABASE_URL", url)
	t.Setenv("MARKUPP_AUTH_SECRET", "")

	require.NoError(t, run(context.Background(), slog.New(slog.DiscardHandler), []string{"markupp", "index"}),
		"o Job do indexador nao recebe o segredo dos JWT")

	var pendentes int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT markupp_pending_vaults()").Scan(&pendentes))
	assert.Zero(t, pendentes)
}

func TestMigrateDatabase_ComSenhaDoScaler_LiberaOLoginDoPapel(t *testing.T) {
	dono := storagetest.DatabaseURL(t)

	require.NoError(t, migrateDatabase(context.Background(), dono, "senha-do-scaler"))

	scaler, err := url.Parse(dono)
	require.NoError(t, err)
	scaler.User = url.UserPassword("markupp_scaler", "senha-do-scaler")
	conn, err := pgx.Connect(context.Background(), scaler.String())
	require.NoError(t, err, "o KEDA entra com o papel do scaler, nao com o dono do banco")
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	var pendentes int
	require.NoError(t, conn.QueryRow(context.Background(), "SELECT markupp_pending_vaults()").Scan(&pendentes))
	assert.Zero(t, pendentes)
}

func TestParseCommand_NotifyLoop_RetornaEnvioEmLaco(t *testing.T) {
	cmd, err := parseCommand([]string{"markupp", "notify-loop"})

	require.NoError(t, err)
	assert.Equal(t, notifyLoopCommand, cmd)
}

func TestRun_NotifyLoopSemSMTP_RecusaNomeandoAVariavel(t *testing.T) {
	t.Setenv("MARKUPP_DATABASE_URL", "postgres://ninguem@127.0.0.1:1/nada?connect_timeout=1")
	t.Setenv("MARKUPP_AUTH_SECRET", "12345678901234567890123456789012")

	err := run(context.Background(), slog.New(slog.DiscardHandler), []string{"markupp", "notify-loop"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_SMTP_URL")
}

func TestRun_NotifyLoopComSMTPEBancoInalcancavel_RetornaErroDoBanco(t *testing.T) {
	t.Setenv("MARKUPP_DATABASE_URL", "postgres://ninguem@127.0.0.1:1/nada?connect_timeout=1")
	t.Setenv("MARKUPP_AUTH_SECRET", "12345678901234567890123456789012")
	t.Setenv("MARKUPP_SMTP_URL", "smtp://127.0.0.1:1")
	t.Setenv("MARKUPP_SMTP_FROM", "convites@markupp.dev")

	err := run(context.Background(), slog.New(slog.DiscardHandler), []string{"markupp", "notify-loop"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "abrir banco")
}

func TestOAuthMetadataFrom_SemPublicURL_DesligaAMetadata(t *testing.T) {
	assert.Nil(t, oauthMetadataFrom(config.Config{}))
}

func TestOAuthMetadataFrom_ComPublicURL_UsaComoIssuer(t *testing.T) {
	cfg := config.Config{PublicURL: "https://notas.markupp.dev"}

	meta := oauthMetadataFrom(cfg)

	require.NotNil(t, meta)
	assert.Equal(t, "https://notas.markupp.dev", meta.Issuer)
}

func TestOAuthServiceConfigFrom_ConverteDuracoesEListaOsResources(t *testing.T) {
	cfg := config.Config{
		PublicURL: "https://notas.markupp.dev", PanelURL: "https://painel.markupp.dev",
		OAuthResources:     []config.OAuthResource{{URL: "https://mcp.markupp.dev", Secret: "s"}},
		OAuthAccessMinutes: 60, OAuthRefreshDays: 30, OAuthRequestMinutes: 10, OAuthCodeSeconds: 60,
	}

	got := oauthServiceConfigFrom(cfg)

	assert.Equal(t, "https://notas.markupp.dev", got.Issuer)
	assert.Equal(t, "https://painel.markupp.dev", got.PanelURL)
	assert.Equal(t, []string{"https://mcp.markupp.dev"}, got.Resources)
	assert.Equal(t, time.Hour, got.AccessTTL)
	assert.Equal(t, 30*24*time.Hour, got.RefreshTTL)
	assert.Equal(t, 10*time.Minute, got.RequestTTL)
	assert.Equal(t, time.Minute, got.CodeTTL)
}

func TestOAuthServiceConfigFrom_CacheNegativoDoCIMDUsaOMinimoDoCache(t *testing.T) {
	got := oauthServiceConfigFrom(config.Config{OAuthCIMDCacheMinMinutes: 7})

	assert.Equal(t, 7*time.Minute, got.CIMDFailureTTL)
}

func TestOAuthServiceConfigFrom_MapeiaOsSegredosEAValidadeDaTroca(t *testing.T) {
	cfg := config.Config{
		OAuthResources: []config.OAuthResource{
			{URL: "https://mcp.markupp.dev/mcp", Secret: "segredo-um"}, // #nosec G101 -- valor fictício de teste
			{URL: "https://outro.markupp.dev", Secret: "segredo-dois"}, // #nosec G101 -- valor fictício de teste
		},
		OAuthExchangeMinutes: 5,
	}

	got := oauthServiceConfigFrom(cfg)

	assert.Equal(t, map[string]string{
		"https://mcp.markupp.dev/mcp": "segredo-um", "https://outro.markupp.dev": "segredo-dois",
	}, got.ResourceSecrets)
	assert.Equal(t, 5*time.Minute, got.ExchangeTTL)
}

func TestNewOAuthService_SemPublicURL_DevolveNilDeInterface(t *testing.T) {
	svc, err := newOAuthService(config.Config{}, nil, nil)

	require.NoError(t, err)
	assert.Nil(t, svc)
}

func TestCIMDLimitsFrom_ConverteAConfigEmDuracoesEBytes(t *testing.T) {
	cfg := config.Config{
		OAuthCIMDTimeoutSeconds: 5, OAuthCIMDMaxKB: 64, OAuthCIMDCacheMinMinutes: 5, OAuthCIMDCacheMaxHours: 24,
	}

	assert.Equal(t, cimdLimits{
		Timeout: 5 * time.Second, MaxBodyBytes: 64 << 10, MinTTL: 5 * time.Minute, MaxTTL: 24 * time.Hour,
	}, cimdLimitsFrom(cfg))
}

// fakeCIMDServer publica um Client ID Metadata Document em https no loopback.
type fakeCIMDServer struct {
	*httptest.Server
	status int
	delay  time.Duration
}

func newFakeCIMDServer(t *testing.T, status int) *fakeCIMDServer {
	srv := &fakeCIMDServer{status: status}
	srv.Server = httptest.NewTLSServer(http.HandlerFunc(srv.serveDocument))
	t.Cleanup(srv.Close)
	return srv
}

func (s *fakeCIMDServer) clientID() string { return s.URL + "/cliente.json" }

// allowedPort libera no fetcher a porta aleatória do servidor de teste.
func (s *fakeCIMDServer) allowedPort(t *testing.T) cimd.Option {
	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	return cimd.WithAllowedPort(u.Port())
}

func (s *fakeCIMDServer) roots() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(s.Certificate())
	return pool
}

func (s *fakeCIMDServer) serveDocument(w http.ResponseWriter, r *http.Request) {
	select {
	case <-time.After(s.delay):
	case <-r.Context().Done():
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=60")
	w.WriteHeader(s.status)
	_ = json.NewEncoder(w).Encode(struct {
		ClientID     string   `json:"client_id"`
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
	}{s.clientID(), "Cliente de Teste", []string{"https://cliente.markupp.dev/cb"}})
}

// allowLoopback libera o servidor de teste e mantém o resto da política padrão.
func allowLoopback(ip netip.Addr) error {
	if ip.IsLoopback() {
		return nil
	}
	return cimd.PublicOnly(ip)
}

var testCIMDLimits = cimdLimits{Timeout: 5 * time.Second, MaxBodyBytes: 64 << 10, MinTTL: 5 * time.Minute, MaxTTL: time.Hour}

func TestCIMDClientMetadataFetcher_DocumentoValido_ConverteOsMetadadosComOsLimites(t *testing.T) {
	srv := newFakeCIMDServer(t, http.StatusOK)
	fetcher, err := newCIMDClientMetadataFetcher(testCIMDLimits, cimd.WithRootCAs(srv.roots()), srv.allowedPort(t), cimd.WithIPPolicy(allowLoopback))
	require.NoError(t, err)

	meta, err := fetcher.FetchClientMetadata(context.Background(), srv.clientID())

	require.NoError(t, err)
	assert.Equal(t, oauth.ClientMetadata{
		ClientID: srv.clientID(), Name: "Cliente de Teste",
		RedirectURIs: []string{"https://cliente.markupp.dev/cb"}, TTL: 5 * time.Minute,
	}, meta, "o max-age de 60 s sobe ao minimo de 5 min dos limites")
}

func TestCIMDClientMetadataFetcher_BuscaFalha_EmbrulhaOErroComOClientID(t *testing.T) {
	srv := newFakeCIMDServer(t, http.StatusNotFound)
	fetcher, err := newCIMDClientMetadataFetcher(testCIMDLimits, cimd.WithRootCAs(srv.roots()), srv.allowedPort(t), cimd.WithIPPolicy(allowLoopback))
	require.NoError(t, err)

	_, err = fetcher.FetchClientMetadata(context.Background(), srv.clientID())

	require.ErrorIs(t, err, cimd.ErrFetchFailed)
	assert.Contains(t, err.Error(), oauth.ClientValue(srv.clientID()))
}

func TestNewCIMDClientMetadataFetcher_SemTrocarAPolitica_RecusaLoopback(t *testing.T) {
	srv := newFakeCIMDServer(t, http.StatusOK)
	fetcher, err := newCIMDClientMetadataFetcher(testCIMDLimits, cimd.WithRootCAs(srv.roots()), srv.allowedPort(t))
	require.NoError(t, err)

	_, err = fetcher.FetchClientMetadata(context.Background(), srv.clientID())

	require.ErrorIs(t, err, cimd.ErrBlockedAddress, "esperada a politica PublicOnly")
}

func TestNewCIMDClientMetadataFetcher_LimitesInvalidos_RetornaErro(t *testing.T) {
	_, err := newCIMDClientMetadataFetcher(cimdLimits{Timeout: time.Second, MaxBodyBytes: 1, MinTTL: time.Hour, MaxTTL: time.Minute})

	require.ErrorIs(t, err, cimd.ErrInvalidOption)
}

func TestNewCIMDClientMetadataFetcher_LimiteDeCorpo_ChegaAoFetcher(t *testing.T) {
	srv := newFakeCIMDServer(t, http.StatusOK)
	limits := testCIMDLimits
	limits.MaxBodyBytes = 16
	fetcher, err := newCIMDClientMetadataFetcher(limits, cimd.WithRootCAs(srv.roots()), srv.allowedPort(t), cimd.WithIPPolicy(allowLoopback))
	require.NoError(t, err)

	_, err = fetcher.FetchClientMetadata(context.Background(), srv.clientID())

	require.ErrorIs(t, err, cimd.ErrFetchFailed)
	assert.Contains(t, err.Error(), "mais de 16 bytes")
}

func TestNewCIMDClientMetadataFetcher_Timeout_ChegaAoFetcher(t *testing.T) {
	srv := newFakeCIMDServer(t, http.StatusOK)
	srv.delay = 2 * time.Second
	limits := testCIMDLimits
	limits.Timeout = 100 * time.Millisecond
	fetcher, err := newCIMDClientMetadataFetcher(limits, cimd.WithRootCAs(srv.roots()), srv.allowedPort(t), cimd.WithIPPolicy(allowLoopback))
	require.NoError(t, err)
	inicio := time.Now()

	_, err = fetcher.FetchClientMetadata(context.Background(), srv.clientID())

	require.ErrorIs(t, err, cimd.ErrFetchFailed)
	assert.Less(t, time.Since(inicio), time.Second, "o timeout dos limites encerra a busca antes do atraso")
}
