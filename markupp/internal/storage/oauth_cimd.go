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

// SaveCIMDClient cria ou atualiza o cliente CIMD com o documento buscado em now e válido
// até expiresAt e, na mesma transação, apaga os clientes CIMD vencidos em now sem
// Autorização ativa, a mesma regra do DCR; as revogadas vão junto em cascata. Recusa, sem
// gravar nem apagar, o id já registrado por outra origem.
func (r *PostgresOAuthRepository) SaveCIMDClient(ctx context.Context, client oauth.Client, expiresAt, now time.Time) error {
	return inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		if err := upsertCIMDClientInTx(ctx, qtx, client, expiresAt); err != nil {
			return err
		}
		if err := qtx.DeleteStaleCIMDClients(ctx, now); err != nil {
			return fmt.Errorf("apagar clientes cimd vencidos em %s: %w", now.Format(time.RFC3339), err)
		}
		return nil
	})
}

// upsertCIMDClientInTx grava o cliente CIMD, sem sobrescrever o de outra origem.
func upsertCIMDClientInTx(ctx context.Context, qtx *gen.Queries, client oauth.Client, expiresAt time.Time) error {
	rows, err := qtx.UpsertCIMDClient(ctx, gen.UpsertCIMDClientParams{
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
