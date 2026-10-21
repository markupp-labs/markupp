package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresInstanceSecrets guarda os segredos que o servidor gera na primeira subida
// (ADR-0041). A tabela é da instância: sem ator nem RLS. O valor nunca entra em erro.
type PostgresInstanceSecrets struct {
	q *gen.Queries
}

// NewPostgresInstanceSecrets monta o repositório de segredos sobre um pool aberto.
func NewPostgresInstanceSecrets(pool *pgxpool.Pool) *PostgresInstanceSecrets {
	return &PostgresInstanceSecrets{q: gen.New(pool)}
}

// Secret devolve o segredo name, gerando-o com generate na primeira vez. Entre réplicas
// subindo juntas vale o primeiro valor gravado, e as outras leem esse.
func (s *PostgresInstanceSecrets) Secret(ctx context.Context, name string, generate func() (string, error)) (string, error) {
	candidate, err := generate()
	if err != nil {
		return "", fmt.Errorf("gerar segredo da instancia %s: %w", name, err)
	}
	err = s.q.InsertInstanceSecretIfMissing(ctx, gen.InsertInstanceSecretIfMissingParams{Name: name, Value: candidate})
	if err != nil {
		return "", fmt.Errorf("gravar segredo da instancia %s: %w", name, err)
	}
	value, err := s.q.GetInstanceSecret(ctx, name)
	if err != nil {
		return "", fmt.Errorf("ler segredo da instancia %s: %w", name, err)
	}
	return value, nil
}
