package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func TestListMyNotifications_ConvitePendenteViraNotificacaoDeConvite(t *testing.T) {
	expira := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepository(defaultClock())
	repo.pendingForUser = []auth.PendingInvite{{ID: "c1", TenantID: "t2", TenantName: "Home lab", Role: "member", ExpiresAt: expira}}
	svc := setupAuthService(repo, false, defaultClock())

	feed, err := svc.ListMyNotifications(context.Background(), "u1")

	require.NoError(t, err)
	assert.Equal(t, auth.NotificationFeed{
		Items:  []auth.Notification{{Kind: auth.NotificationKindInvite, ID: "c1", TenantID: "t2", TenantName: "Home lab", Role: "member", ExpiresAt: expira}},
		Unread: 1,
	}, feed)
}

func TestListMyNotifications_SemConvite_DevolveListaVaziaENadaPendente(t *testing.T) {
	svc := setupAuthService(newFakeRepository(defaultClock()), false, defaultClock())

	feed, err := svc.ListMyNotifications(context.Background(), "u1")

	require.NoError(t, err)
	assert.NotNil(t, feed.Items, "lista vazia, nao nula, para o JSON sair como []")
	assert.Empty(t, feed.Items)
	assert.Zero(t, feed.Unread)
}

func TestListMyNotifications_FalhaAoListarConvites_DevolveOErroComAConta(t *testing.T) {
	repo := newFakeRepository(defaultClock())
	repo.pendingErr = errors.New("banco fora")
	svc := setupAuthService(repo, false, defaultClock())

	_, err := svc.ListMyNotifications(context.Background(), "u1")

	require.ErrorIs(t, err, repo.pendingErr)
	assert.Contains(t, err.Error(), `"u1"`)
}
