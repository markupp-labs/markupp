package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
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

	db, err := storage.OpenDB(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("abrir db %q: %w", cfg.DBPath, err)
	}
	defer func() { _ = db.Close() }()

	if err := storage.Migrate(db); err != nil {
		return fmt.Errorf("aplicar migrations: %w", err)
	}

	addr := ":" + strconv.Itoa(cfg.Port)
	logger.Info("servidor subindo", "addr", addr)
	return http.ListenAndServe(addr, newHandler(cfg, db))
}

func newHandler(cfg config.Config, db *sql.DB) http.Handler {
	repo := storage.NewSqliteNotesRepository(db)
	return api.NewRouter(notes.NewService(repo, cfg.MaxNoteSize))
}
