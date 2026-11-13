package auth_test

import (
	"cmp"
	"context"
	"slices"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// ListUserIdentities imita a ordem do banco: por criação e, no empate, por id.
func (f *fakeRepository) ListUserIdentities(_ context.Context, userID string) ([]auth.UserIdentity, error) {
	var out []auth.UserIdentity
	for _, ident := range f.identities {
		if ident.UserID == userID {
			out = append(out, ident)
		}
	}
	slices.SortFunc(out, func(a, b auth.UserIdentity) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (f *fakeRepository) LinkIdentityTx(_ context.Context, ident auth.UserIdentity, event auth.AuditEventRecord) error {
	key := ident.Provider + ":" + ident.ProviderUserID
	if _, taken := f.identities[key]; taken {
		return auth.ErrIdentityAlreadyLinked
	}
	f.identities[key] = ident
	f.auditEvents = append(f.auditEvents, event)
	return nil
}

func (f *fakeRepository) UnlinkIdentityTx(ctx context.Context, userID, identityID string, event auth.AuditEventRecord) error {
	identities, _ := f.ListUserIdentities(ctx, userID)
	i := slices.IndexFunc(identities, func(ident auth.UserIdentity) bool { return ident.ID == identityID })
	if i < 0 {
		return auth.ErrIdentityNotFound
	}
	if len(identities) == 1 {
		return auth.ErrLastIdentity
	}
	delete(f.identities, identities[i].Provider+":"+identities[i].ProviderUserID)
	f.auditEvents = append(f.auditEvents, event)
	return nil
}
