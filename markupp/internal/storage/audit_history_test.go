package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

var inicioDoHistorico = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

// historicoSemeado grava, com a origem de cada pedido, quatro eventos: login e falha da
// dona, troca de senha da u2 e falha de email sem conta.
func historicoSemeado(t *testing.T) (*pgxpool.Pool, *storage.PostgresAuditHistory) {
	t.Helper()
	db := setupTestDB(t)
	_, err := db.Exec(context.Background(), `INSERT INTO users (id, email, created_at, updated_at) VALUES
		('dona', 'dona@markupp.dev', now(), now()), ('u2', 'u2@markupp.dev', now(), now())`)
	require.NoError(t, err)
	repo := storage.NewPostgresAuthRepository(db)
	eventos := []struct {
		origem auth.Origin
		evento auth.AuditEventRecord
	}{
		{auth.Origin{IP: "203.0.113.7", UserAgent: "Firefox/140"}, auth.AuditEventRecord{ID: "e1", ActorID: "dona", Action: "auth.login", Target: "dona@markupp.dev", Result: "success", OccurredAt: inicioDoHistorico}},
		{auth.Origin{IP: "198.51.100.9", UserAgent: "curl/8"}, auth.AuditEventRecord{ID: "e2", ActorID: "dona", Action: "auth.login", Target: "dona@markupp.dev", Result: "failure", OccurredAt: inicioDoHistorico.Add(time.Hour)}},
		{auth.Origin{}, auth.AuditEventRecord{ID: "e3", ActorID: "u2", Action: "auth.password_reset", Target: "u2", Result: "success", OccurredAt: inicioDoHistorico.Add(2 * time.Hour)}},
		{auth.Origin{IP: "198.51.100.9"}, auth.AuditEventRecord{ID: "e4", Action: "auth.login", Target: "ninguem@markupp.dev", Result: "failure", OccurredAt: inicioDoHistorico.Add(3 * time.Hour)}},
	}
	for _, e := range eventos {
		require.NoError(t, repo.RecordAuditEvent(auth.WithOrigin(context.Background(), e.origem), e.evento))
	}
	return db, storage.NewPostgresAuditHistory(db)
}

func idsDoHistorico(entries []auth.AuditEntry) []string {
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.ID)
	}
	return ids
}

func TestAuditHistory_GuardaAOrigemEListaDoMaisNovo(t *testing.T) {
	_, repo := historicoSemeado(t)

	got, err := repo.ListAuditEvents(context.Background(), auth.AuditQuery{Limit: 50})

	require.NoError(t, err)
	assert.Equal(t, []string{"e4", "e3", "e2", "e1"}, idsDoHistorico(got))
	primeiro := got[3]
	assert.Equal(t, "dona", primeiro.ActorID)
	assert.Equal(t, "dona@markupp.dev", primeiro.ActorEmail)
	assert.Equal(t, "auth.login", primeiro.Action)
	assert.Equal(t, "success", primeiro.Result)
	assert.Equal(t, "203.0.113.7", primeiro.IP)
	assert.Equal(t, "Firefox/140", primeiro.UserAgent)
	assert.True(t, inicioDoHistorico.Equal(primeiro.OccurredAt))
	assert.Empty(t, got[0].ActorEmail, "falha de email sem conta não tem autor")
}

func TestAuditHistory_FiltraContaAcaoResultadoPeriodoEPagina(t *testing.T) {
	_, repo := historicoSemeado(t)
	ctx := context.Background()
	desde, ate := inicioDoHistorico.Add(30*time.Minute), inicioDoHistorico.Add(150*time.Minute)

	daDona, err := repo.ListAuditEvents(ctx, auth.AuditQuery{Account: "dona", Limit: 50})
	require.NoError(t, err)
	falhas, err := repo.ListAuditEvents(ctx, auth.AuditQuery{Action: "auth.login", Result: "failure", Limit: 50})
	require.NoError(t, err)
	periodo, err := repo.ListAuditEvents(ctx, auth.AuditQuery{Since: &desde, Until: &ate, Limit: 50})
	require.NoError(t, err)
	pagina, err := repo.ListAuditEvents(ctx, auth.AuditQuery{Offset: 1, Limit: 2})
	require.NoError(t, err)
	porIP, err := repo.ListAuditEvents(ctx, auth.AuditQuery{IP: "198.51.100.9", Limit: 50})
	require.NoError(t, err)

	assert.Equal(t, []string{"e2", "e1"}, idsDoHistorico(daDona))
	assert.Equal(t, []string{"e4", "e2"}, idsDoHistorico(falhas))
	assert.Equal(t, []string{"e3", "e2"}, idsDoHistorico(periodo))
	assert.Equal(t, []string{"e3", "e2"}, idsDoHistorico(pagina))
	assert.Equal(t, []string{"e4", "e2"}, idsDoHistorico(porIP))
}
