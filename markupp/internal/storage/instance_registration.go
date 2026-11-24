package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresInstanceRegistration lê e grava a escolha do cadastro por método (ADR-0059). A
// gravação é do administrador, no papel markupp_app; a leitura serve também ao cadastro,
// que não tem ator.
type PostgresInstanceRegistration struct {
	pool *pgxpool.Pool
}

var _ instance.RegistrationStore = (*PostgresInstanceRegistration)(nil)

// NewInstanceRegistration monta o repositório sobre um pool aberto.
func NewInstanceRegistration(pool *pgxpool.Pool) *PostgresInstanceRegistration {
	return &PostgresInstanceRegistration{pool: pool}
}

// LoadRegistrationChoices devolve a escolha guardada de cada método.
func (r *PostgresInstanceRegistration) LoadRegistrationChoices(ctx context.Context) (map[string]bool, error) {
	rows, err := gen.New(r.pool).ListInstanceRegistrationMethods(ctx)
	if err != nil {
		return nil, fmt.Errorf("read instance registration methods: %w", err)
	}
	choices := make(map[string]bool, len(rows))
	for _, row := range rows {
		choices[row.Method] = row.Open
	}
	return choices, nil
}

// SaveRegistrationChoice grava se o método aceita cadastro.
func (r *PostgresInstanceRegistration) SaveRegistrationChoice(ctx context.Context, method string, open bool) error {
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		return q.SaveInstanceRegistrationMethod(ctx, gen.SaveInstanceRegistrationMethodParams{Method: method, Open: open})
	})
	if err != nil {
		return fmt.Errorf("save registration method %q as open=%t: %w", method, open, err)
	}
	return nil
}

// HasAccounts diz se já há conta na instalação, como o cadastro confere antes de criar.
func (r *PostgresInstanceRegistration) HasAccounts(ctx context.Context) (bool, error) {
	count, err := gen.New(r.pool).CountUsers(ctx)
	if err != nil {
		return false, fmt.Errorf("count accounts of the instance: %w", err)
	}
	return count > 0, nil
}
