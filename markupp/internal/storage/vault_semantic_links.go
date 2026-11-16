package storage

import (
	"context"
	"fmt"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// refreshVaultSemanticLinks refaz as ligações por sentido do cofre pelo kNN mútuo dos
// vetores gravados (ADR-0053). Cofre sem vetor, ou instalação sem provedor, fica sem.
func refreshVaultSemanticLinks(ctx context.Context, q *gen.Queries, vaultID string) error {
	if err := q.DeleteVaultSemanticLinks(ctx, vaultID); err != nil {
		return fmt.Errorf("delete semantic links of vault %q: %w", vaultID, err)
	}
	if err := q.AllowFilteredVectorScan(ctx); err != nil {
		return fmt.Errorf("allow filtered vector scan for vault %q: %w", vaultID, err)
	}
	if err := q.SaveVaultSemanticLinks(ctx, vaultID); err != nil {
		return fmt.Errorf("save semantic links of vault %q: %w", vaultID, err)
	}
	return nil
}
