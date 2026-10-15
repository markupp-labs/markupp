// Command markupp sobe o servidor HTTP de notas, ou aplica as migrações do
// banco com o subcomando migrate.
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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
)

// command é o subcomando do binário: serve sobe a API, migrate aplica o schema
// e sai.
type command string

const (
	serveCommand   command = "serve"
	migrateCommand command = "migrate"
)

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
	if cmd == migrateCommand {
		return migrateDatabase(ctx, cfg.DatabaseURL)
	}
	return serve(ctx, logger, cfg)
}

func parseCommand(args []string) (command, error) {
	if len(args) < 2 {
		return serveCommand, nil
	}
	switch cmd := command(args[1]); cmd {
	case serveCommand, migrateCommand:
		return cmd, nil
	}
	return "", fmt.Errorf("subcomando %q desconhecido, esperado %q ou %q", args[1], serveCommand, migrateCommand)
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
	return api.NewRouterWithAuth(notesSvc, authSvc, codec, pool, cfg.AllowedOrigins)
}

func newAuthService(cfg config.Config, pool *pgxpool.Pool) (api.AuthService, api.TokenValidator) {
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
