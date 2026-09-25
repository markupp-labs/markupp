// Command markupp sobe o servidor HTTP de notas: carrega a configuração,
// abre e migra o banco, e serve a API REST.
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

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("servidor encerrado", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}

	ctx := context.Background()
	pool, err := storage.OpenPool(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}
	defer pool.Close()

	if err := storage.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("aplicar migrations: %w", err)
	}

	server := newServer(cfg.Port, newHandler(cfg, pool))
	logger.Info("servidor subindo", "addr", server.Addr)
	return server.ListenAndServe()
}

func newHandler(cfg config.Config, pool *pgxpool.Pool) http.Handler {
	repo := storage.NewPostgresNotesRepository(pool)
	svc := notes.NewService(repo, cfg.MaxNoteSize)
	return api.NewRouter(svc, pool, cfg.AllowedOrigins)
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
