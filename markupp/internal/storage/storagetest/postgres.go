// Package storagetest sobe um PostgreSQL descartável para os testes que
// precisam do banco de verdade. Um contêiner por processo de teste, e um banco
// novo por chamada, para que nenhum teste enxergue a escrita de outro.
package storagetest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const postgresImage = "postgres:17.9-alpine3.23"

var (
	containerOnce sync.Once
	adminURL      string
	errContainer  error
	databaseSeq   atomic.Int64
)

// DatabaseURL cria um banco vazio no contêiner compartilhado e devolve a URL
// de conexão para ele.
func DatabaseURL(t *testing.T) string {
	t.Helper()
	admin := sharedAdminURL(t)
	name := fmt.Sprintf("teste_%d_%d", os.Getpid(), databaseSeq.Add(1))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, admin)
	require.NoError(t, err)
	defer func() { _ = conn.Close(ctx) }()
	_, err = conn.Exec(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)

	return withDatabase(t, admin, name)
}

// EmptyPool devolve um pool sobre um banco novo e sem schema, fechado ao fim
// do teste.
func EmptyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), DatabaseURL(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func sharedAdminURL(t *testing.T) string {
	t.Helper()
	containerOnce.Do(func() { adminURL, errContainer = startContainer() })
	require.NoError(t, errContainer, "subir o postgres de teste exige Docker disponivel")
	return adminURL
}

func startContainer() (string, error) {
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, postgresImage,
		tcpostgres.WithDatabase("markupp"),
		tcpostgres.WithUsername("markupp"),
		tcpostgres.WithPassword("markupp"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return "", fmt.Errorf("subir contêiner %s: %w", postgresImage, err)
	}
	return ctr.ConnectionString(ctx, "sslmode=disable")
}

func withDatabase(t *testing.T, rawURL, name string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	require.NoError(t, err, "url do contêiner fora do formato postgres://: %q", rawURL)
	parsed.Path = "/" + name
	return parsed.String()
}
