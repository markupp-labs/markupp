package storage

import (
	"context"
	"fmt"
	"slices"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// ListUserIdentities devolve as formas de entrar da conta, na ordem em que nasceram.
func (r *PostgresAuthRepository) ListUserIdentities(ctx context.Context, userID string) ([]auth.UserIdentity, error) {
	return listUserIdentities(ctx, r.q, userID)
}

func listUserIdentities(ctx context.Context, q *gen.Queries, userID string) ([]auth.UserIdentity, error) {
	rows, err := q.ListUserIdentities(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list identities of user %q: %w", userID, err)
	}
	identities := make([]auth.UserIdentity, 0, len(rows))
	for _, row := range rows {
		identities = append(identities, toAuthIdentity(row))
	}
	return identities, nil
}

// LinkIdentityTx vincula a identidade externa à conta e audita. Identidade já usada por
// qualquer conta sai como auth.ErrIdentityAlreadyLinked.
func (r *PostgresAuthRepository) LinkIdentityTx(ctx context.Context, ident auth.UserIdentity, event auth.AuditEventRecord) error {
	return r.inTx(ctx, func(qtx *gen.Queries) error {
		err := qtx.CreateUserIdentity(ctx, toCreateIdentityParams(ident))
		if isUniqueConstraintViolation(err) {
			return fmt.Errorf("%w: %s:%s", auth.ErrIdentityAlreadyLinked, ident.Provider, ident.ProviderUserID)
		}
		if err != nil {
			return fmt.Errorf("link identity %s to user %q: %w", ident.Provider, ident.UserID, err)
		}
		return qtx.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
}

// UnlinkIdentityTx desvincula a identidade da conta com a conta travada, recusando a
// última, e audita.
func (r *PostgresAuthRepository) UnlinkIdentityTx(ctx context.Context, userID, identityID string, event auth.AuditEventRecord) error {
	return r.inTx(ctx, func(qtx *gen.Queries) error {
		if err := qtx.LockUserIdentities(ctx, userID); err != nil {
			return fmt.Errorf("lock identities of user %q: %w", userID, err)
		}
		if err := checkUnlinkable(ctx, qtx, userID, identityID); err != nil {
			return err
		}
		if err := qtx.DeleteUserIdentity(ctx, gen.DeleteUserIdentityParams{ID: identityID, UserID: userID}); err != nil {
			return fmt.Errorf("unlink identity %q of user %q: %w", identityID, userID, err)
		}
		return qtx.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
}

// checkUnlinkable exige que a identidade seja da conta e que outra reste depois.
func checkUnlinkable(ctx context.Context, q *gen.Queries, userID, identityID string) error {
	identities, err := listUserIdentities(ctx, q, userID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(identities, func(i auth.UserIdentity) bool { return i.ID == identityID }) {
		return fmt.Errorf("%w: identity %q of user %q", auth.ErrIdentityNotFound, identityID, userID)
	}
	if len(identities) == 1 {
		return fmt.Errorf("%w: identity %q of user %q", auth.ErrLastIdentity, identityID, userID)
	}
	return nil
}
