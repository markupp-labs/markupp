package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

var hojeDaLimpeza = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func limpezaDeHoje() instance.RetentionCleanup {
	return instance.RetentionCleanup{Cutoff: hojeDaLimpeza.AddDate(0, 0, -30), Now: hojeDaLimpeza, DueAfter: hojeDaLimpeza.Add(-24 * time.Hour)}
}

// historicoParaLimpar grava um evento velho e um novo, uma sessão encerrada há muito, uma
// encerrada ontem e uma ativa antiga, todas com IP.
func historicoParaLimpar(t *testing.T) *pgxpool.Pool {
	t.Helper()
	db := setupTestDB(t)
	_, err := db.Exec(context.Background(), `
		INSERT INTO users (id, email, created_at, updated_at) VALUES ('u1', 'u1@markupp.dev', now(), now());
		INSERT INTO audit_events (id, actor_id, action, target, result, occurred_at, client_ip)
		VALUES ('velho', 'u1', 'auth.login', 'u1', 'success', '2026-09-01T00:00:00Z', '203.0.113.7'),
		       ('novo', 'u1', 'auth.login', 'u1', 'success', '2026-11-20T00:00:00Z', '203.0.113.7');
		INSERT INTO user_sessions (id, user_id, client_ip, user_agent, created_at, last_used_at, ended_at) VALUES
		('encerrada-velha', 'u1', '203.0.113.7', 'Firefox', '2026-08-01', '2026-08-01', '2026-09-01T00:00:00Z'),
		('encerrada-ontem', 'u1', '203.0.113.7', 'Firefox', '2026-08-01', '2026-08-01', '2026-11-24T00:00:00Z'),
		('ativa-antiga', 'u1', '203.0.113.7', 'Firefox', '2026-08-01', '2026-08-01', NULL)`)
	require.NoError(t, err)
	return db
}

func contar(t *testing.T, db *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(context.Background(), sql).Scan(&n))
	return n
}

func TestRetention_EscolhaDoAdministrador_GuardaEAudita(t *testing.T) {
	db := bancoDaInstancia(t)
	repo := storage.NewInstanceRetention(db)
	evento := auth.AuditEventRecord{ID: "ev-ret", ActorID: "admin", Action: "instance.retention_update", Target: "30",
		Result: "success", OccurredAt: hojeDaLimpeza}

	antes, err := repo.LoadRetentionDays(context.Background())
	require.NoError(t, err)
	require.NoError(t, repo.SaveRetentionDays(context.Background(), 30, evento))
	depois, err := repo.LoadRetentionDays(context.Background())
	require.NoError(t, err)

	assert.Zero(t, antes, "sem escolha")
	assert.Equal(t, 30, depois)
	assert.Equal(t, 1, contar(t, db, `SELECT count(*) FROM audit_events WHERE id = 'ev-ret'`))
}

func TestRetention_Limpeza_ApagaOVelhoETiraOIPDasSessoesEncerradasHaMuito(t *testing.T) {
	db := historicoParaLimpar(t)

	result, err := storage.NewInstanceRetention(db).CleanupIfDue(context.Background(), limpezaDeHoje())

	require.NoError(t, err)
	assert.Equal(t, instance.RetentionResult{Ran: true, AuditEvents: 1, Origins: 1}, result)
	assert.Equal(t, 0, contar(t, db, `SELECT count(*) FROM audit_events WHERE id = 'velho'`))
	assert.Equal(t, 1, contar(t, db, `SELECT count(*) FROM audit_events WHERE id = 'novo'`))
	assert.Equal(t, 0, contar(t, db, `SELECT count(*) FROM user_sessions WHERE id = 'encerrada-velha' AND (client_ip <> '' OR user_agent <> '')`))
	assert.Equal(t, 1, contar(t, db, `SELECT count(*) FROM user_sessions WHERE id = 'encerrada-ontem' AND client_ip <> ''`))
	assert.Equal(t, 1, contar(t, db, `SELECT count(*) FROM user_sessions WHERE id = 'ativa-antiga' AND client_ip <> ''`),
		"a sessão ativa guarda o IP enquanto está ativa")
}

func TestRetention_SegundaLimpezaNoMesmoDia_NaoRoda(t *testing.T) {
	db := historicoParaLimpar(t)
	repo := storage.NewInstanceRetention(db)
	_, err := repo.CleanupIfDue(context.Background(), limpezaDeHoje())
	require.NoError(t, err)
	_, err = db.Exec(context.Background(), `INSERT INTO audit_events (id, action, target, result, occurred_at)
		VALUES ('velho-2', 'auth.login', 'x', 'failure', '2026-09-02T00:00:00Z')`)
	require.NoError(t, err)

	maisTarde := limpezaDeHoje()
	maisTarde.Now = maisTarde.Now.Add(time.Hour)
	result, err := repo.CleanupIfDue(context.Background(), maisTarde)

	require.NoError(t, err)
	assert.False(t, result.Ran)
	assert.Equal(t, 1, contar(t, db, `SELECT count(*) FROM audit_events WHERE id = 'velho-2'`))
}

func TestRetention_ForaDaLimpeza_HistoricoContinuaInsertOnly(t *testing.T) {
	db := historicoParaLimpar(t)

	_, err := db.Exec(context.Background(), `DELETE FROM audit_events WHERE id = 'velho'`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert-only")
}
