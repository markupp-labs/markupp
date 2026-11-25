package instance_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
)

type fakeRetentionStore struct {
	days    int
	saved   *int
	event   auth.AuditEventRecord
	cleanup *instance.RetentionCleanup
	result  instance.RetentionResult
}

func (f *fakeRetentionStore) LoadRetentionDays(context.Context) (int, error) { return f.days, nil }

func (f *fakeRetentionStore) SaveRetentionDays(_ context.Context, days int, event auth.AuditEventRecord) error {
	f.saved, f.event = &days, event
	return nil
}

func (f *fakeRetentionStore) CleanupIfDue(_ context.Context, c instance.RetentionCleanup) (instance.RetentionResult, error) {
	f.cleanup = &c
	return f.result, nil
}

var agoraDaRetencao = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func retencao(store *fakeRetentionStore, hostDays int) *instance.RetentionService {
	return instance.NewRetentionService(store, hostDays, func() time.Time { return agoraDaRetencao }, func() string { return "ev-1" })
}

func TestRetention_SemEscolhaNemVariavel_365Livre(t *testing.T) {
	view, err := retencao(&fakeRetentionStore{}, 0).Settings(context.Background())

	require.NoError(t, err)
	assert.Equal(t, instance.RetentionView{Days: 365, HostLocked: false}, view)
}

func TestRetention_EscolhaDoAdministrador_Vale(t *testing.T) {
	view, err := retencao(&fakeRetentionStore{days: 30}, 0).Settings(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 30, view.Days)
}

func TestRetention_VariavelDefinida_VenceETrava(t *testing.T) {
	store := &fakeRetentionStore{days: 30}
	svc := retencao(store, 90)

	view, err := svc.Settings(context.Background())
	require.NoError(t, err)
	_, updateErr := svc.Update(context.Background(), "admin", 10)

	assert.Equal(t, instance.RetentionView{Days: 90, HostLocked: true}, view)
	require.ErrorIs(t, updateErr, instance.ErrHostLocked)
	assert.Nil(t, store.saved)
}

func TestRetention_Atualizar_GuardaEAudita(t *testing.T) {
	store := &fakeRetentionStore{}

	view, err := retencao(store, 0).Update(context.Background(), "admin", 30)

	require.NoError(t, err)
	assert.Equal(t, 30, *store.saved)
	assert.Equal(t, 30, view.Days)
	assert.Equal(t, auth.AuditEventRecord{ID: "ev-1", ActorID: "admin", Action: "instance.retention_update",
		Target: "30", Result: "success", OccurredAt: agoraDaRetencao}, store.event)
}

func TestRetention_AtualizarForaDaFaixa_Recusa(t *testing.T) {
	for _, dias := range []int{0, 3651} {
		_, err := retencao(&fakeRetentionStore{}, 0).Update(context.Background(), "admin", dias)

		require.ErrorIs(t, err, instance.ErrInvalidRetention, dias)
		assert.Contains(t, err.Error(), "from 1 to 3650")
	}
}

func TestRetention_Limpar_UsaOPrazoVigenteEUmDiaEntreLimpezas(t *testing.T) {
	store := &fakeRetentionStore{days: 30, result: instance.RetentionResult{Ran: true, AuditEvents: 4, Origins: 2}}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	require.NoError(t, retencao(store, 0).CleanupIfDue(context.Background(), logger))

	require.NotNil(t, store.cleanup)
	assert.Equal(t, instance.RetentionCleanup{Cutoff: agoraDaRetencao.AddDate(0, 0, -30), Now: agoraDaRetencao,
		DueAfter: agoraDaRetencao.Add(-24 * time.Hour)}, *store.cleanup)
}
