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

func eventoDaInstancia(action, target string) auth.AuditEventRecord {
	return auth.AuditEventRecord{ID: "ev-" + action + "-" + target, ActorID: "admin", Action: action, Target: target,
		Result: "success", OccurredAt: time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)}
}

// contasSemeadas semeia dona (única owner de t1, com duas notas, e member de t2), u2
// (member de t1) e outra (owner de t2).
func contasSemeadas(t *testing.T) (*pgxpool.Pool, *storage.PostgresInstanceAccountsRepository) {
	t.Helper()
	db := setupTestDB(t)
	semearDoisTenants(t, db)
	return db, storage.NewPostgresInstanceAccountsRepository(db)
}

func idsDe(list []auth.AccountSummary) []string {
	ids := make([]string, 0, len(list))
	for _, a := range list {
		ids = append(ids, a.ID)
	}
	return ids
}

func TestInstanceAccounts_Listar_TodasDaMaisNova(t *testing.T) {
	db, repo := contasSemeadas(t)
	_, err := db.Exec(context.Background(), `UPDATE users SET created_at = now() - (interval '1 day' *
		CASE id WHEN 'dona' THEN 3 WHEN 'u2' THEN 2 ELSE 1 END)`)
	require.NoError(t, err)

	got, err := repo.ListAccounts(context.Background(), auth.AccountQuery{Limit: 50})

	require.NoError(t, err)
	assert.Equal(t, []string{"outra", "u2", "dona"}, idsDe(got))
	assert.Equal(t, "dona@markupp.dev", got[2].Email)
	assert.Empty(t, got[2].LoginMethods)
	assert.Nil(t, got[2].LastAccessAt, "conta sem sessão nem app com uso")
}

func TestInstanceAccounts_Listar_FiltraEmailStatusEPagina(t *testing.T) {
	db, repo := contasSemeadas(t)
	_, err := db.Exec(context.Background(), `UPDATE users SET blocked_at = now() WHERE id = 'u2'`)
	require.NoError(t, err)

	porEmail, err := repo.ListAccounts(context.Background(), auth.AccountQuery{Email: "DON", Limit: 50})
	require.NoError(t, err)
	bloqueadas, err := repo.ListAccounts(context.Background(), auth.AccountQuery{Status: "blocked", Limit: 50})
	require.NoError(t, err)
	pagina, err := repo.ListAccounts(context.Background(), auth.AccountQuery{Offset: 1, Limit: 1})
	require.NoError(t, err)

	assert.Equal(t, []string{"dona"}, idsDe(porEmail), "parte do email, sem diferenciar maiúsculas")
	assert.Equal(t, []string{"u2"}, idsDe(bloqueadas))
	assert.True(t, bloqueadas[0].Blocked)
	assert.Len(t, pagina, 1)
}

func TestInstanceAccounts_Listar_MetodosEUltimoAcesso(t *testing.T) {
	db, repo := contasSemeadas(t)
	ctx := context.Background()
	_, err := db.Exec(ctx, `
		INSERT INTO user_identities (id, user_id, provider, provider_user_id, created_at)
		VALUES ('i1', 'dona', 'local', 'dona@markupp.dev', now()), ('i2', 'dona', 'github', '42', now());
		INSERT INTO user_sessions (id, user_id, created_at, last_used_at)
		VALUES ('s1', 'dona', now(), '2026-11-20T10:00:00Z'), ('s2', 'dona', now(), '2026-11-24T10:00:00Z')`)
	require.NoError(t, err)

	got, err := repo.ListAccounts(ctx, auth.AccountQuery{Email: "dona", Limit: 50})

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, []string{"github", "local"}, got[0].LoginMethods)
	require.NotNil(t, got[0].LastAccessAt)
	assert.True(t, time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC).Equal(*got[0].LastAccessAt))
}

func TestInstanceAccounts_Detalhe_DizOQueSomeComAConta(t *testing.T) {
	_, repo := contasSemeadas(t)

	got, err := repo.GetAccountDetail(context.Background(), "dona")

	require.NoError(t, err)
	assert.Equal(t, "dona@markupp.dev", got.Email)
	assert.Equal(t, []auth.OwnedTenant{{ID: "t1", Name: "t1", Notes: 2}}, got.SoleOwnerTenants,
		"t2 tem outra owner e fica")
}

func TestInstanceAccounts_Detalhe_Inexistente(t *testing.T) {
	_, repo := contasSemeadas(t)

	_, err := repo.GetAccountDetail(context.Background(), "ninguem")

	require.ErrorIs(t, err, auth.ErrUserNotFound)
}

func TestInstanceAccounts_Bloquear_EncerraSessoesERevogaApps(t *testing.T) {
	db, repo := contasSemeadas(t)
	ctx := context.Background()
	_, err := db.Exec(ctx, `
		INSERT INTO user_sessions (id, user_id, created_at, last_used_at) VALUES ('s1', 'dona', now(), now());
		INSERT INTO refresh_tokens (token_hash, user_id, session_id, expires_at, created_at)
		VALUES ('h1', 'dona', 's1', now() + interval '1 day', now());
		INSERT INTO oauth_clients (id, source, name, redirect_uris) VALUES ('c1', 'dcr', 'Agente', ARRAY['http://127.0.0.1/cb']);
		INSERT INTO oauth_authorizations (id, user_id, client_id, scopes) VALUES ('a1', 'dona', 'c1', '{notas:leitura}');
		INSERT INTO oauth_tokens (token_hash, authorization_id, kind, resource, scopes, expires_at)
		VALUES ('t1', 'a1', 'refresh', 'https://x', '{notas:leitura}', now() + interval '1 day')`)
	require.NoError(t, err)

	require.NoError(t, repo.SetAccountBlocked(ctx, "dona", true, eventoDaInstancia("accounts.block", "dona")))

	var bloqueada, sessaoEncerrada, refreshRevogado, appRevogado, tokenRevogado, auditado bool
	require.NoError(t, db.QueryRow(ctx, `SELECT
		(SELECT blocked_at IS NOT NULL FROM users WHERE id = 'dona'),
		(SELECT ended_at IS NOT NULL FROM user_sessions WHERE id = 's1'),
		(SELECT revoked FROM refresh_tokens WHERE token_hash = 'h1'),
		(SELECT revoked_at IS NOT NULL FROM oauth_authorizations WHERE id = 'a1'),
		(SELECT revoked FROM oauth_tokens WHERE token_hash = 't1'),
		EXISTS (SELECT 1 FROM audit_events WHERE id = 'ev-accounts.block-dona')`).
		Scan(&bloqueada, &sessaoEncerrada, &refreshRevogado, &appRevogado, &tokenRevogado, &auditado))
	assert.True(t, bloqueada)
	assert.True(t, sessaoEncerrada)
	assert.True(t, refreshRevogado)
	assert.True(t, appRevogado)
	assert.True(t, tokenRevogado)
	assert.True(t, auditado)
}

func TestInstanceAccounts_Desbloquear_LimpaOBloqueio(t *testing.T) {
	db, repo := contasSemeadas(t)
	ctx := context.Background()
	require.NoError(t, repo.SetAccountBlocked(ctx, "dona", true, eventoDaInstancia("accounts.block", "dona")))

	require.NoError(t, repo.SetAccountBlocked(ctx, "dona", false, eventoDaInstancia("accounts.unblock", "dona")))

	var bloqueada bool
	require.NoError(t, db.QueryRow(ctx, `SELECT blocked_at IS NOT NULL FROM users WHERE id = 'dona'`).Scan(&bloqueada))
	assert.False(t, bloqueada)
}

func TestInstanceAccounts_BloquearInexistente_NaoAcha(t *testing.T) {
	_, repo := contasSemeadas(t)

	err := repo.SetAccountBlocked(context.Background(), "ninguem", true, eventoDaInstancia("accounts.block", "ninguem"))

	require.ErrorIs(t, err, auth.ErrUserNotFound)
}

func TestInstanceAccounts_Apagar_LevaOsTenantsSemOutraOwner(t *testing.T) {
	db, repo := contasSemeadas(t)
	ctx := context.Background()

	require.NoError(t, repo.DeleteAccount(ctx, "dona", eventoDaInstancia("accounts.delete", "dona")))

	var conta, t1, t2, notasDeT1, u2, notaDeT2 int
	require.NoError(t, db.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM users WHERE id = 'dona'),
		(SELECT count(*) FROM tenants WHERE id = 't1'),
		(SELECT count(*) FROM tenants WHERE id = 't2'),
		(SELECT count(*) FROM notes WHERE tenant_id = 't1'),
		(SELECT count(*) FROM users WHERE id = 'u2'),
		(SELECT count(*) FROM notes WHERE tenant_id = 't2')`).
		Scan(&conta, &t1, &t2, &notasDeT1, &u2, &notaDeT2))
	assert.Zero(t, conta)
	assert.Zero(t, t1, "t1 só tinha a dona como owner")
	assert.Zero(t, notasDeT1)
	assert.Equal(t, 1, t2, "t2 tem outra owner")
	assert.Equal(t, 1, notaDeT2)
	assert.Equal(t, 1, u2, "o membro de t1 continua com a conta")
}

func TestInstanceAccounts_ApagarInexistente_NaoAcha(t *testing.T) {
	_, repo := contasSemeadas(t)

	err := repo.DeleteAccount(context.Background(), "ninguem", eventoDaInstancia("accounts.delete", "ninguem"))

	require.ErrorIs(t, err, auth.ErrUserNotFound)
}
