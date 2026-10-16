package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/ifsc-ES2/projeto-markupp/markupp/migrations"
)

// OpenPool abre o pool de conexões em databaseURL e confirma com um ping.
func OpenPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("configurar pool, esperado postgres://usuario:senha@host/banco: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Migrate aplica as migrações embutidas em migrations.FS. Rodar de novo sem
// migração pendente não muda nada.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

// EnableScalerLogin dá LOGIN com password ao papel markupp_scaler, que só conta os
// cofres pendentes: o KEDA consulta o banco sem a credencial do dono. O próprio
// banco cita a senha, porque ALTER ROLE não aceita parâmetro.
func EnableScalerLogin(ctx context.Context, pool *pgxpool.Pool, password string) error {
	var stmt string
	err := pool.QueryRow(ctx, "SELECT format('ALTER ROLE markupp_scaler WITH LOGIN PASSWORD %L', $1::text)", password).Scan(&stmt)
	if err != nil {
		return fmt.Errorf("montar o ALTER ROLE do papel markupp_scaler: %w", err)
	}
	if _, err := pool.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("dar LOGIN ao papel markupp_scaler, esperado dono do banco com CREATEROLE: %w", err)
	}
	return nil
}
