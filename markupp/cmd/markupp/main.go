// Command markupp sobe o servidor HTTP de notas, aplica as migrações do banco
// com o subcomando migrate, indexa os cofres pendentes com index (um cofre, para
// o Job do KEDA) e index-loop (em laço, para o compose), ou reenfileira notas para
// reindexar com reindex.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/drain"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notify"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
)

// command é o subcomando do binário: serve sobe a API, migrate aplica o schema
// e sai, index indexa um cofre pendente e sai, index-loop indexa em laço, e
// notify-loop entrega a fila de email em laço, e reindex reenfileira as notas de
// um cofre ou da instalação.
type command string

const (
	serveCommand      command = "serve"
	migrateCommand    command = "migrate"
	indexCommand      command = "index"
	indexLoopCommand  command = "index-loop"
	notifyLoopCommand command = "notify-loop"
	reindexCommand    command = "reindex"
)

// notifyInterval é a espera do notify-loop com a fila vazia. A primeira nova
// tentativa de envio é de um minuto, então meio minuto basta.
const notifyInterval = 30 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(context.Background(), logger, os.Args); err != nil {
		logger.Error("servidor encerrado", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, args []string) error {
	cmd, err := parseCommand(args)
	if err != nil {
		return err
	}
	if cmd == indexCommand || cmd == indexLoopCommand {
		return runIndexer(ctx, logger, cmd)
	}
	if cmd == reindexCommand {
		return reindexVaults(ctx, logger, args[2:])
	}
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	switch cmd {
	case migrateCommand:
		return migrateDatabase(ctx, cfg.DatabaseURL, cfg.ScalerPassword)
	case notifyLoopCommand:
		return deliverNotifications(ctx, logger, cfg)
	}
	return serve(ctx, logger, cfg)
}

func parseCommand(args []string) (command, error) {
	if len(args) < 2 {
		return serveCommand, nil
	}
	switch cmd := command(args[1]); cmd {
	case serveCommand, migrateCommand, indexCommand, indexLoopCommand, notifyLoopCommand, reindexCommand:
		return cmd, nil
	}
	return "", fmt.Errorf("subcomando %q desconhecido, esperado %q, %q, %q, %q, %q ou %q",
		args[1], serveCommand, migrateCommand, indexCommand, indexLoopCommand, notifyLoopCommand, reindexCommand)
}

const reindexUsage = "esperado reindex --vault <id> para um cofre ou reindex --all para a instalacao"

// parseReindexScope lê o escopo do reindex: devolve o cofre de --vault, ou vazio com
// --all. Exige exatamente um dos dois, para a instalação inteira nunca ser o padrão.
func parseReindexScope(args []string) (string, error) {
	flags := flag.NewFlagSet(string(reindexCommand), flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	vaultID := flags.String("vault", "", "cofre a reindexar")
	all := flags.Bool("all", false, "reindexa todos os cofres da instalacao")
	if err := flags.Parse(args); err != nil {
		return "", fmt.Errorf("argumentos %q do reindex invalidos, %s: %w", args, reindexUsage, err)
	}
	if flags.NArg() > 0 || (*vaultID != "") == *all {
		return "", fmt.Errorf("argumentos %q do reindex, %s", args, reindexUsage)
	}
	return *vaultID, nil
}

// reindexVaults reenfileira as notas do escopo para o indexador refazer o índice,
// como depois de trocar MARKUPP_SEARCH_CONFIG. Usa a config do indexador: só
// precisa do banco, com o papel dono.
func reindexVaults(ctx context.Context, logger *slog.Logger, args []string) error {
	vaultID, err := parseReindexScope(args)
	if err != nil {
		return err
	}
	cfg, err := config.LoadIndexer(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("carregar config do indexador: %w", err)
	}
	pool, err := storage.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}
	defer pool.Close()
	requeued, err := storage.RequeueNotes(ctx, pool, vaultID)
	if err != nil {
		return err
	}
	logger.Info("reindexacao enfileirada", "cofre", vaultID, "notas", requeued)
	return nil
}

// deliverNotifications entrega a fila de email em laço: é o Deployment que o KEDA
// escala entre zero e uma réplica, e o serviço do compose.
func deliverNotifications(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
	if cfg.SMTPURL == "" {
		return fmt.Errorf("MARKUPP_SMTP_URL vazia, esperado smtp://usuario:senha@host:porta para entregar a fila de email")
	}
	mailer, err := notify.NewSMTPMailer(cfg.SMTPURL)
	if err != nil {
		return err
	}
	pool, err := storage.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}
	defer pool.Close()
	deliverer := notify.NewDeliverer(storage.NewPostgresNotificationOutbox(pool), mailer, cfg.SMTPFrom, cfg.PanelURL, auth.NewOpaqueToken, time.Now)
	logger.Info("notificador em laco", "intervalo", notifyInterval.String())
	return drain.Run(ctx, deliverer.DeliverNext, drain.WaitInterval(notifyInterval))
}

// runIndexer para no SIGTERM do Kubernetes ou do compose: a transação aberta volta
// atrás e o cofre fica para o próximo Job.
func runIndexer(ctx context.Context, logger *slog.Logger, cmd command) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.LoadIndexer(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("carregar config do indexador: %w", err)
	}
	return indexVaults(ctx, logger, cfg, cmd)
}

// indexVaults roda o indexador sem estado do pacote indexing (ADR-0033): index
// atende o Job que o KEDA cria por cofre, index-loop atende o compose.
func indexVaults(ctx context.Context, logger *slog.Logger, cfg config.Config, cmd command) error {
	pool, err := storage.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}
	defer pool.Close()
	indexer, err := storage.NewPostgresVaultIndexer(ctx, pool, cfg.SearchConfig, logger)
	if err != nil {
		return err
	}
	if cmd == indexLoopCommand {
		logger.Info("indexador em laco", "intervalo", cfg.IndexInterval.String())
		return indexing.RunLoop(ctx, indexer, drain.WaitInterval(cfg.IndexInterval), logger)
	}
	found, err := indexer.IndexNextVault(ctx, indexing.Index)
	logger.Info("indexacao de um cofre", "cofre_pendente", found)
	return err
}

// migrateDatabase aplica as migrações e, com scalerPassword, libera o login do
// papel do scaler do KEDA.
func migrateDatabase(ctx context.Context, databaseURL, scalerPassword string) error {
	pool, err := storage.OpenPool(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}
	defer pool.Close()
	if err := storage.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("aplicar migrations: %w", err)
	}
	if scalerPassword == "" {
		return nil
	}
	return storage.EnableScalerLogin(ctx, pool, scalerPassword)
}

// serve sobe a API sem mexer no schema: quem migra é o subcomando migrate.
func serve(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
	pool, err := storage.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}
	defer pool.Close()

	cfg, err = prepareInstance(ctx, cfg, storage.NewPostgresInstanceSecrets(pool))
	if err != nil {
		return fmt.Errorf("preparar a instancia: %w", err)
	}
	handler, err := newHandler(cfg, pool, logger)
	if err != nil {
		return err
	}
	server := newServer(cfg.Port, handler)
	logger.Info("servidor subindo", "addr", server.Addr)
	return server.ListenAndServe()
}

func newHandler(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) (http.Handler, error) {
	repo := storage.NewPostgresNotesRepository(pool, cfg.SearchConfig)
	notesSvc := notes.NewService(repo, cfg.MaxNoteSize)
	authSvc, codec := newAuthService(cfg, pool)
	// A API não recebe o SMTP: o booleano diz que há quem entregue os emails dela.
	emailEnabled := cfg.InviteByEmail
	adminSvc := auth.NewAdminService(storage.NewPostgresAdminRepository(pool), time.Now, emailEnabled)
	accountEmail := auth.NewAccountEmailService(storage.NewPostgresAccountEmailRepository(pool), auth.NewBcryptHasher(), time.Now, auth.AccountEmailOptions{
		EmailEnabled: emailEnabled, LocalEnabled: cfg.LocalAuthEnabled, Background: runDetached, Logger: logger,
	})
	oauthSvc, err := newOAuthService(cfg, pool, codec)
	if err != nil {
		return nil, err
	}
	deps := api.RouterDeps{
		Notes: notesSvc, Graph: storage.NewPostgresNoteGraph(pool, cfg.SearchConfig), Auth: authSvc, Admin: adminSvc, AccountEmail: accountEmail,
		Tokens: codec, Tenants: authSvc, Probe: pool, AllowedOrigins: cfg.AllowedOrigins,
		OAuthMetadata: oauthMetadataFrom(cfg), OAuth: oauthSvc, MCPUpstreamURL: cfg.MCPUpstreamURL,
		AccountRateLimiter: api.NewAccountRateLimiter(cfg.RateLimitPerMinute, time.Minute, time.Now, cfg.TrustedProxies),
	}
	addVaultPublication(&deps, cfg, pool)
	return api.NewRouter(deps), nil
}

// addVaultPublication liga a publicação dos cofres e a leitura sem conta deles
// (ADR-0048), com limite por IP próprio.
func addVaultPublication(deps *api.RouterDeps, cfg config.Config, pool *pgxpool.Pool) {
	deps.Publications = publication.NewService(storage.NewPostgresVaultPublications(pool), time.Now)
	deps.PublicVaults = storage.NewPostgresPublicVaults(pool, cfg.SearchConfig)
	deps.PublicRateLimiter = api.NewAccountRateLimiter(cfg.PublicRateLimitPerMinute, time.Minute, time.Now, cfg.TrustedProxies)
	// O servidor MCP se autentica com o segredo de resource dele e repassa o IP do agente
	// de /mcp/public (ADR-0051). Sem OAuth não há resource, e o limite fica na conexão.
	deps.PublicRelay, _ = deps.OAuth.(api.RelayAuthenticator)
}

// O *oauth.Service confere o servidor MCP da leitura pública.
var _ api.RelayAuthenticator = (*oauth.Service)(nil)

// newOAuthService monta o authorization server só com o OAuth ligado. Devolve nil de
// interface, e não um *oauth.Service nil, para o roteador não montar as rotas.
func newOAuthService(cfg config.Config, pool *pgxpool.Pool, scoped oauth.ScopedTokenIssuer) (api.OAuthService, error) {
	if !cfg.OAuthEnabled() {
		return nil, nil
	}
	metadata, err := newCIMDClientMetadataFetcher(cimdLimitsFrom(cfg))
	if err != nil {
		return nil, err
	}
	repo := storage.NewPostgresOAuthRepository(pool)
	return oauth.NewService(repo, oauthServiceConfigFrom(cfg), scoped, metadata, time.Now, uuid.NewString, newOAuthToken), nil
}

// newOAuthToken gera os tokens opacos do OAuth. A leitura de crypto/rand não falha num
// sistema saudável, então a falha derruba o processo em vez de emitir token fraco.
func newOAuthToken() string {
	token, err := auth.NewOpaqueToken()
	if err != nil {
		panic(fmt.Sprintf("oauth: gerar token opaco: %v", err))
	}
	return token
}

// oauthServiceConfigFrom traduz a config do processo para o serviço OAuth.
func oauthServiceConfigFrom(cfg config.Config) oauth.ServiceConfig {
	resources := make([]string, 0, len(cfg.OAuthResources))
	secrets := make(map[string]string, len(cfg.OAuthResources))
	for _, resource := range cfg.OAuthResources {
		resources = append(resources, resource.URL)
		secrets[resource.URL] = resource.Secret
	}
	return oauth.ServiceConfig{
		Issuer: cfg.PublicURL, PanelURL: cfg.PanelURL, Resources: resources, ResourceSecrets: secrets,
		AccessTTL:   time.Duration(cfg.OAuthAccessMinutes) * time.Minute,
		RefreshTTL:  time.Duration(cfg.OAuthRefreshDays) * 24 * time.Hour,
		RequestTTL:  time.Duration(cfg.OAuthRequestMinutes) * time.Minute,
		CodeTTL:     time.Duration(cfg.OAuthCodeSeconds) * time.Second,
		ExchangeTTL: time.Duration(cfg.OAuthExchangeMinutes) * time.Minute,
		// A falha da busca fica lembrada pelo menor tempo de cache do documento.
		CIMDFailureTTL: time.Duration(cfg.OAuthCIMDCacheMinMinutes) * time.Minute,
		DCRMaxPending:  cfg.OAuthDCRMaxPending,
		DCRPendingTTL:  time.Duration(cfg.OAuthDCRPendingHours) * time.Hour,
	}
}

// oauthMetadataFrom publica a metadata do authorization server só quando a
// MARKUPP_PUBLIC_URL liga o OAuth, com o registro dinâmico conforme a config.
func oauthMetadataFrom(cfg config.Config) *api.OAuthMetadata {
	if !cfg.OAuthEnabled() {
		return nil
	}
	return &api.OAuthMetadata{Issuer: cfg.PublicURL, DynamicRegistration: cfg.OAuthDCREnabled}
}

func newAuthService(cfg config.Config, pool *pgxpool.Pool) (*auth.Service, *auth.TokenCodec) {
	authRepo := storage.NewPostgresAuthRepository(pool)
	codec := auth.NewTokenCodec(cfg.AuthSecret, time.Duration(cfg.JWTExpirationMinutes)*time.Minute, time.Now)
	hasher := auth.NewBcryptHasher()
	authCfg := auth.ServiceConfig{
		DefaultTenantID:         cfg.DefaultTenantID,
		RegistrationEnabled:     cfg.AuthRegistrationEnabled,
		LocalEnabled:            cfg.LocalAuthEnabled,
		RefreshExpirationPeriod: time.Duration(cfg.RefreshExpirationDays) * 24 * time.Hour,
		EmailEnabled:            cfg.InviteByEmail,
	}
	return auth.NewService(authRepo, codec, hasher, authCfg, time.Now), codec
}

func newServer(port int, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// runDetached roda f fora da requisição, para o pedido de troca de senha responder
// antes de gravar.
func runDetached(f func()) { go f() }
