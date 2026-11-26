package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

type fakeAuditHistoryRepository struct {
	query auth.AuditQuery
}

func (f *fakeAuditHistoryRepository) ListAuditEvents(_ context.Context, q auth.AuditQuery) ([]auth.AuditEntry, error) {
	f.query = q
	return nil, nil
}

func TestAuditHistory_SemLimite_Pagina50(t *testing.T) {
	repo := &fakeAuditHistoryRepository{}

	_, err := auth.NewAuditHistory(repo).List(context.Background(), auth.AuditQuery{Account: "u1"})

	require.NoError(t, err)
	assert.Equal(t, auth.AuditQuery{Account: "u1", Limit: 50}, repo.query)
}

func TestAuditHistory_PeriodoInvertido_Recusa(t *testing.T) {
	desde := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	ate := desde.Add(-time.Hour)

	_, err := auth.NewAuditHistory(&fakeAuditHistoryRepository{}).List(context.Background(), auth.AuditQuery{Since: &desde, Until: &ate})

	require.ErrorIs(t, err, auth.ErrInvalidAuditQuery)
	assert.Contains(t, err.Error(), "expected")
}

func TestAuditHistory_ResultadoDesconhecido_Recusa(t *testing.T) {
	_, err := auth.NewAuditHistory(&fakeAuditHistoryRepository{}).List(context.Background(), auth.AuditQuery{Result: "talvez"})

	require.ErrorIs(t, err, auth.ErrInvalidAuditQuery)
	assert.Contains(t, err.Error(), `"talvez"`)
}
