package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// GetFreshCIMDClient devolve o cliente CIMD cujo documento em cache ainda vale em now,
// ou oauth.ErrClientNotFound se não existe, venceu ou é de outra origem.
func (r *PostgresOAuthRepository) GetFreshCIMDClient(ctx context.Context, id string, now time.Time) (oauth.Client, error) {
	row, err := r.q.GetFreshCIMDClient(ctx, gen.GetFreshCIMDClientParams{ID: id, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return oauth.Client{}, fmt.Errorf("%w: client_id cimd %s sem documento válido", oauth.ErrClientNotFound, oauth.ClientValue(id))
	}
	if err != nil {
		return oauth.Client{}, fmt.Errorf("buscar cliente cimd %s: %w", oauth.ClientValue(id), err)
	}
	return oauth.Client{ID: row.ID, Name: row.Name, RedirectURIs: row.RedirectUris}, nil
}

// SaveCIMDClient cria ou atualiza o cliente CIMD com o documento válido até expiresAt.
// Recusa, sem gravar, o id já registrado por outra origem.
func (r *PostgresOAuthRepository) SaveCIMDClient(ctx context.Context, client oauth.Client, expiresAt time.Time) error {
	rows, err := r.q.UpsertCIMDClient(ctx, gen.UpsertCIMDClientParams{
		ID: client.ID, Name: client.Name, RedirectUris: client.RedirectURIs, ExpiresAt: expiresAt,
	})
	if err != nil {
		return fmt.Errorf("salvar cliente cimd %s: %w", oauth.ClientValue(client.ID), err)
	}
	if rows == 0 {
		return fmt.Errorf(`salvar cliente cimd %s: %w, esperado source "cimd"`, oauth.ClientValue(client.ID), oauth.ErrClientSourceConflict)
	}
	return nil
}
