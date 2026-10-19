package storage

import (
	"context"
	"fmt"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// RegisterDCRClient grava o cliente DCR numa transação que trava todos os registros da
// instância, expira os pendentes vencidos e recusa o registro com o teto atingido.
func (r *PostgresOAuthRepository) RegisterDCRClient(ctx context.Context, reg oauth.DCRRegistration) error {
	return inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		if err := qtx.LockDCRRegistrations(ctx); err != nil {
			return fmt.Errorf("travar os registros dcr: %w", err)
		}
		if err := reservePendingDCRSlot(ctx, qtx, reg); err != nil {
			return err
		}
		return createDCRClient(ctx, qtx, reg)
	})
}

// reservePendingDCRSlot apaga os pendentes criados há PendingTTL ou mais e devolve
// *oauth.PendingClientLimitError se os que sobram já somam MaxPending.
func reservePendingDCRSlot(ctx context.Context, qtx *gen.Queries, reg oauth.DCRRegistration) error {
	if err := qtx.DeleteExpiredPendingDCRClients(ctx, reg.IssuedAt.Add(-reg.PendingTTL)); err != nil {
		return fmt.Errorf("expirar os clientes dcr pendentes: %w", err)
	}
	pending, err := qtx.CountPendingDCRClients(ctx, reg.IssuedAt)
	if err != nil {
		return fmt.Errorf("contar os clientes dcr pendentes: %w", err)
	}
	if pending.Pending < int64(reg.MaxPending) {
		return nil
	}
	return &oauth.PendingClientLimitError{MaxPending: reg.MaxPending, FreesAt: pending.OldestCreatedAt.Add(reg.PendingTTL)}
}

func createDCRClient(ctx context.Context, qtx *gen.Queries, reg oauth.DCRRegistration) error {
	err := qtx.CreateDCRClient(ctx, gen.CreateDCRClientParams{
		ID: reg.Client.ID, Name: reg.Client.Name, RedirectUris: reg.Client.RedirectURIs, CreatedAt: reg.IssuedAt,
	})
	if err != nil {
		return fmt.Errorf("gravar cliente dcr %s: %w", oauth.ClientValue(reg.Client.ID), err)
	}
	return nil
}
