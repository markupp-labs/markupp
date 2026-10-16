// Command markupp sobe o servidor HTTP de notas, aplica as migrações do banco
// com o subcomando migrate, ou indexa os cofres pendentes com index (um cofre, para
// o Job do KEDA) e index-loop (em laço, para o compose).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notify"
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
// notify-loop entrega a fila de email em laço.
type command string

const (
	serveCommand      command = "serve"
	migrateCommand    command = "migrate"
	indexCommand      command = "index"
	indexLoopCommand  command = "index-loop"
	notifyLoopCommand command = "notify-loop"
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
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	switch cmd {
	case migrateCommand:
		return migrateDatabase(ctx, cfg.DatabaseURL)
	case indexCommand, indexLoopCommand:
		return indexVaults(ctx, logger, cfg, cmd)
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
	case serveCommand, migrateCommand, indexCommand, indexLoopCommand, notifyLoopCommand:
		return cmd, nil
	}
	return "", fmt.Errorf("subcomando %q desconhecido, esperado %q, %q, %q, %q ou %q",
		args[1], serveCommand, migrateCommand, indexCommand, indexLoopCommand, notifyLoopCommand)
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
	deliverer := notify.NewDeliverer(mailer, cfg.SMTPFrom, auth.NewOpaqueToken, time.Now)
	logger.Info("notificador em laco", "intervalo", notifyInterval.String())
	return notify.RunLoop(ctx, storage.NewPostgresInviteOutbox(pool), deliverer.Deliver, indexing.WaitInterval(notifyInterval))
}

// indexVaults roda o indexador sem estado do pacote indexing (ADR-0033): index
// atende o Job que o KEDA cria por cofre, index-loop atende o compose.
func indexVaults(ctx context.Context, logger *slog.Logger, cfg config.Config, cmd command) error {
	pool, err := storage.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}
	defer pool.Close()
	indexer, err := storage.NewPostgresVaultIndexer(ctx, pool, cfg.SearchConfig)
	if err != nil {
		return err
	}
	if cmd == indexLoopCommand {
		logger.Info("indexador em laco", "intervalo", cfg.IndexInterval.String())
		return indexing.RunLoop(ctx, indexer, indexing.WaitInterval(cfg.IndexInterval))
	}
	found, err := indexer.IndexNextVault(ctx, indexing.Index)
	logger.Info("indexacao de um cofre", "cofre_pendente", found)
	return err
}

func migrateDatabase(ctx context.Context, databaseURL string) error {
	pool, err := storage.OpenPool(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}
	defer pool.Close()
	if err := storage.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("aplicar migrations: %w", err)
	}
	return nil
}

// serve sobe a API sem mexer no schema: quem migra é o subcomando migrate.
func serve(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
	pool, err := storage.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}
	defer pool.Close()

	server := newServer(cfg.Port, newHandler(cfg, pool))
	logger.Info("servidor subindo", "addr", server.Addr)
	return server.ListenAndServe()
}

func newHandler(cfg config.Config, pool *pgxpool.Pool) http.Handler {
	repo := storage.NewPostgresNotesRepository(pool)
	notesSvc := notes.NewService(repo, cfg.MaxNoteSize)
	authSvc, codec := newAuthService(cfg, pool)
	adminSvc := auth.NewAdminService(storage.NewPostgresAdminRepository(pool), time.Now, false)
	return api.NewRouter(api.RouterDeps{
		Notes: notesSvc, Auth: authSvc, Admin: adminSvc,
		Tokens: codec, Tenants: authSvc, Probe: pool, AllowedOrigins: cfg.AllowedOrigins,
	})
}

func newAuthService(cfg config.Config, pool *pgxpool.Pool) (*auth.Service, api.TokenValidator) {
	authRepo := storage.NewPostgresAuthRepository(pool)
	codec := auth.NewTokenCodec(cfg.AuthSecret, time.Duration(cfg.JWTExpirationMinutes)*time.Minute, time.Now)
	hasher := auth.NewBcryptHasher()
	authCfg := auth.ServiceConfig{
		DefaultTenantID:         cfg.DefaultTenantID,
		RegistrationEnabled:     cfg.AuthRegistrationEnabled,
		LocalEnabled:            cfg.LocalAuthEnabled,
		RefreshExpirationPeriod: time.Duration(cfg.RefreshExpirationDays) * 24 * time.Hour,
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
