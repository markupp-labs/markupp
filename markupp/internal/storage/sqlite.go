package storage

import (
	"database/sql"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // driver sqlite sem cgo, registrado no database/sql

	"github.com/ifsc-ES2/projeto-markupp/markupp/migrations"
)

// OpenDB abre o banco em path e confirma a conexão com um ping.
func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// Migrate aplica as migrações embutidas em migrations.FS.
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Up(db, ".")
}
