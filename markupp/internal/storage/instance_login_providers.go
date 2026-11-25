package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresInstanceLoginProviders guarda os provedores de login da instância (ADR-0060). O
// administrador grava no papel markupp_app; o login, que não tem ator, lê como o dono.
type PostgresInstanceLoginProviders struct {
	pool *pgxpool.Pool
}

var _ instance.LoginProviderStore = (*PostgresInstanceLoginProviders)(nil)

// NewInstanceLoginProviders monta o repositório sobre um pool aberto.
func NewInstanceLoginProviders(pool *pgxpool.Pool) *PostgresInstanceLoginProviders {
	return &PostgresInstanceLoginProviders{pool: pool}
}

// ListLoginProviders devolve os provedores na ordem de criação.
func (r *PostgresInstanceLoginProviders) ListLoginProviders(ctx context.Context) ([]instance.LoginProviderRecord, error) {
	rows, err := gen.New(r.pool).ListInstanceLoginProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("read instance login providers: %w", err)
	}
	records := make([]instance.LoginProviderRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, instance.LoginProviderRecord{
			ID: row.ID, Name: row.Name, Issuer: row.Issuer, ClientID: row.ClientID, SealedSecret: row.ClientSecretSealed,
			Enabled: row.Enabled, LinkedAccounts: int(row.LinkedAccounts), UpdatedAt: row.UpdatedAt,
		})
	}
	return records, nil
}

// CreateLoginProvider grava um provedor novo, e recusa id que já existe.
func (r *PostgresInstanceLoginProviders) CreateLoginProvider(ctx context.Context, record instance.LoginProviderRecord) error {
	var created int64
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		var err error
		created, err = q.CreateInstanceLoginProvider(ctx, gen.CreateInstanceLoginProviderParams{
			ID: record.ID, Name: record.Name, Issuer: record.Issuer, ClientID: record.ClientID,
			ClientSecretSealed: record.SealedSecret, Enabled: record.Enabled,
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("create login provider %q: %w", record.ID, err)
	}
	if created == 0 {
		return fmt.Errorf("%w: %q", instance.ErrLoginProviderExists, record.ID)
	}
	return nil
}

// UpdateLoginProvider troca os campos do provedor record.ID.
func (r *PostgresInstanceLoginProviders) UpdateLoginProvider(ctx context.Context, record instance.LoginProviderRecord) error {
	var updated int64
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		var err error
		updated, err = q.UpdateInstanceLoginProvider(ctx, gen.UpdateInstanceLoginProviderParams{
			ID: record.ID, Name: record.Name, Issuer: record.Issuer, ClientID: record.ClientID,
			ClientSecretSealed: record.SealedSecret, Enabled: record.Enabled,
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("update login provider %q: %w", record.ID, err)
	}
	if updated == 0 {
		return fmt.Errorf("%w: %q", instance.ErrLoginProviderNotFound, record.ID)
	}
	return nil
}

// DeleteLoginProvider apaga numa transação o provedor, os vínculos das contas com ele e a
// escolha do cadastro dele. Roda como o dono, porque os vínculos são de todas as contas,
// mas só com o ator do administrador no contexto.
func (r *PostgresInstanceLoginProviders) DeleteLoginProvider(ctx context.Context, id string) (int, error) {
	if _, ok := identity.ActorFrom(ctx); !ok {
		return 0, ErrNoActor
	}
	var deleted, links int64
	err := inTx(ctx, r.pool, func(q *gen.Queries) error {
		var err error
		if deleted, err = q.DeleteInstanceLoginProvider(ctx, id); err != nil || deleted == 0 {
			return err
		}
		if links, err = q.DeleteLoginProviderIdentities(ctx, id); err != nil {
			return err
		}
		return q.DeleteInstanceRegistrationMethod(ctx, id)
	})
	if err != nil {
		return 0, fmt.Errorf("delete login provider %q: %w", id, err)
	}
	if deleted == 0 {
		return 0, fmt.Errorf("%w: %q", instance.ErrLoginProviderNotFound, id)
	}
	return int(links), nil
}

// ResealLoginProviderSecret troca o selo old por fresh, se ele ainda é o guardado.
func (r *PostgresInstanceLoginProviders) ResealLoginProviderSecret(ctx context.Context, id, old, fresh string) error {
	err := gen.New(r.pool).ResealInstanceLoginProviderSecret(ctx, gen.ResealInstanceLoginProviderSecretParams{ID: id, Old: old, Fresh: fresh})
	if err != nil {
		return fmt.Errorf("reseal the secret of login provider %q: %w", id, err)
	}
	return nil
}
