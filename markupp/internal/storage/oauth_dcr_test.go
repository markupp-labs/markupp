package storage_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

const prazoDosPendentes = 24 * time.Hour

// repoDCR sobe o banco do repoOAuth com o cliente c1 com Autorização ativa, para ele
// não contar como pendente.
func repoDCR(t *testing.T) (*storage.PostgresOAuthRepository, *pgxpool.Pool) {
	t.Helper()
	repo, pool := repoOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", false))
	return repo, pool
}

// autorizarCliente grava a Autorização id de u1 para o cliente, revogada em revogadaEm
// ou ativa quando revogadaEm é nil.
func autorizarCliente(t *testing.T, pool *pgxpool.Pool, id, clientID string, revogadaEm *time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO oauth_authorizations (id, user_id, client_id, scopes, revoked_at)
		VALUES ($1, 'u1', $2, '{notas:leitura}', $3)`, id, clientID, revogadaEm)
	require.NoError(t, err)
}

func registroDCR(id string, emitidoEm time.Time, teto int) oauth.DCRRegistration {
	return oauth.DCRRegistration{
		Client: oauth.Client{
			ID: id, Name: "Agente " + id, RedirectURIs: []string{"http://127.0.0.1/cb", "meuapp://cb"}, Source: oauth.ClientSourceDCR,
		},
		IssuedAt: emitidoEm, MaxPending: teto, PendingTTL: prazoDosPendentes,
	}
}

// clienteAntigo grava direto um cliente da origem dada criado em criadoEm.
func clienteAntigo(t *testing.T, pool *pgxpool.Pool, id, origem string, criadoEm time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO oauth_clients (id, source, name, redirect_uris, metadata_expires_at, created_at)
		VALUES ($1, $2, 'Antigo', ARRAY['http://127.0.0.1/cb'], CASE WHEN $2 = 'cimd' THEN $3::TIMESTAMPTZ END, $3)`, id, origem, criadoEm)
	require.NoError(t, err)
}

func TestPostgresOAuthRepo_RegisterDCRClient_GravaClienteDCR(t *testing.T) {
	repo, pool := repoDCR(t)
	reg := registroDCR("dcr-1", instanteOAuth, 10)

	require.NoError(t, repo.RegisterDCRClient(context.Background(), reg))

	got, err := repo.GetClient(context.Background(), "dcr-1", time.Time{})
	require.NoError(t, err)
	assert.Equal(t, reg.Client, got)
	assert.Equal(t, 1, countRows(t, pool, `SELECT count(*) FROM oauth_clients WHERE id = 'dcr-1'
		AND source = 'dcr' AND metadata_expires_at IS NULL AND created_at = '2026-10-16T12:00:00Z'`))
}

// clientesEmVoltaDoLimite grava, para instanteOAuth, os pendentes vencidos dcr-vencido
// (criado no limite) e dcr-revogado-vencido (antigo, com a última Autorização revogada no
// limite), os pendentes ainda no prazo dcr-quase (criado depois do limite) e
// dcr-revogado-quase (antigo, com a última revogação depois do limite) e os antigos que
// não são pendentes, dcr-autorizado (com Autorização ativa) e cimd-antigo.
func clientesEmVoltaDoLimite(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	limite := instanteOAuth.Add(-prazoDosPendentes)
	antes, depois := limite.Add(-time.Hour), limite.Add(time.Second)
	clienteAntigo(t, pool, "dcr-vencido", "dcr", limite)
	clienteAntigo(t, pool, "dcr-quase", "dcr", depois)
	clienteAntigo(t, pool, "dcr-autorizado", "dcr", antes)
	clienteAntigo(t, pool, "dcr-revogado-vencido", "dcr", antes)
	clienteAntigo(t, pool, "dcr-revogado-quase", "dcr", antes)
	clienteAntigo(t, pool, "cimd-antigo", "cimd", antes)
	autorizarCliente(t, pool, "a-ativa", "dcr-autorizado", nil)
	autorizarCliente(t, pool, "a-revogada-no-limite", "dcr-revogado-vencido", &limite)
	autorizarCliente(t, pool, "a-revogada-antes", "dcr-revogado-quase", &antes)
	autorizarCliente(t, pool, "a-revogada-depois", "dcr-revogado-quase", &depois)
}

func TestPostgresOAuthRepo_RegisterDCRClient_ApagaSoOsPendentesVencidos(t *testing.T) {
	repo, pool := repoDCR(t)
	clientesEmVoltaDoLimite(t, pool)

	require.NoError(t, repo.RegisterDCRClient(context.Background(), registroDCR("dcr-novo", instanteOAuth, 10)))

	for _, id := range []string{"dcr-vencido", "dcr-revogado-vencido"} {
		assert.Zero(t, clientesComID(t, pool, id), "pendente com o prazo cumprido sai: %s", id)
	}
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations WHERE client_id = 'dcr-revogado-vencido'"),
		"as Autorizações revogadas do pendente saem junto")
	for _, id := range []string{"dcr-quase", "dcr-autorizado", "dcr-revogado-quase", "cimd-antigo", "c1", "dcr-novo"} {
		assert.Equal(t, 1, clientesComID(t, pool, id), id)
	}
}

// TestPostgresOAuthRepo_RegisterDCRClient_TudoRevogadoVoltaAoTetoEExpira prova que o
// cliente com todas as Autorizações revogadas volta a ocupar vaga do teto, com o prazo
// contado da revogação, e sai depois dele levando a Autorização, mas não a auditoria.
func TestPostgresOAuthRepo_RegisterDCRClient_TudoRevogadoVoltaAoTetoEExpira(t *testing.T) {
	repo, pool := repoDCR(t)
	ctx := context.Background()
	require.NoError(t, repo.RegisterDCRClient(ctx, registroDCR("dcr-1", instanteOAuth, 1)))
	autorizarCliente(t, pool, "a-dcr", "dcr-1", nil)
	revogacao := oauth.AuditEvent{Action: oauth.AuditActionRevoke, Result: oauth.AuditResultSuccess, UserID: "u1", ClientID: "dcr-1", OccurredAt: instanteOAuth.Add(time.Hour)}
	require.NoError(t, repo.RevokeAuthorization(ctx, "u1", "a-dcr", revogacao))

	err := repo.RegisterDCRClient(ctx, registroDCR("dcr-2", instanteOAuth.Add(2*time.Hour), 1))

	var teto *oauth.PendingClientLimitError
	require.ErrorAs(t, err, &teto)
	assert.True(t, revogacao.OccurredAt.Add(prazoDosPendentes).Equal(teto.FreesAt), teto.FreesAt)
	require.NoError(t, repo.RegisterDCRClient(ctx, registroDCR("dcr-2", teto.FreesAt, 1)))
	assert.Zero(t, clientesComID(t, pool, "dcr-1"))
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations WHERE id = 'a-dcr'"))
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM audit_events WHERE target = 'dcr-1'"), "a auditoria fica")
}

func TestPostgresOAuthRepo_RegisterDCRClient_TetoAtingido_RecusaSemGravar(t *testing.T) {
	repo, pool := repoDCR(t)
	ctx := context.Background()
	require.NoError(t, repo.RegisterDCRClient(ctx, registroDCR("dcr-1", instanteOAuth, 2)))
	require.NoError(t, repo.RegisterDCRClient(ctx, registroDCR("dcr-2", instanteOAuth.Add(time.Minute), 2)))

	err := repo.RegisterDCRClient(ctx, registroDCR("dcr-3", instanteOAuth.Add(2*time.Minute), 2))

	var teto *oauth.PendingClientLimitError
	require.ErrorAs(t, err, &teto)
	assert.Equal(t, 2, teto.MaxPending)
	assert.True(t, instanteOAuth.Add(prazoDosPendentes).Equal(teto.FreesAt), teto.FreesAt)
	assert.Zero(t, clientesComID(t, pool, "dcr-3"))
}

func TestPostgresOAuthRepo_RegisterDCRClient_PendenteVencidoLiberaAVaga(t *testing.T) {
	repo, pool := repoDCR(t)
	ctx := context.Background()
	require.NoError(t, repo.RegisterDCRClient(ctx, registroDCR("dcr-1", instanteOAuth, 1)))

	require.NoError(t, repo.RegisterDCRClient(ctx, registroDCR("dcr-2", instanteOAuth.Add(prazoDosPendentes), 1)))

	assert.Zero(t, clientesComID(t, pool, "dcr-1"))
	assert.Equal(t, 1, clientesComID(t, pool, "dcr-2"))
}

func TestPostgresOAuthRepo_RegisterDCRClient_AutorizadoNaoOcupaVaga(t *testing.T) {
	repo, pool := repoDCR(t)
	ctx := context.Background()
	require.NoError(t, repo.RegisterDCRClient(ctx, registroDCR("dcr-1", instanteOAuth, 1)))
	_, err := pool.Exec(ctx, `INSERT INTO oauth_authorizations (id, user_id, client_id, scopes)
		VALUES ('a-dcr', 'u1', 'dcr-1', '{notas:leitura}')`)
	require.NoError(t, err)

	require.NoError(t, repo.RegisterDCRClient(ctx, registroDCR("dcr-2", instanteOAuth.Add(time.Minute), 1)))
}

// TestPostgresOAuthRepo_RegisterDCRClient_ConcorrenteRespeitaOTetoExato dispara mais
// registros simultâneos que o teto e confere que só o teto passa.
func TestPostgresOAuthRepo_RegisterDCRClient_ConcorrenteRespeitaOTetoExato(t *testing.T) {
	repo, pool := repoDCR(t)
	const teto, tentativas = 5, 20
	erros := make(chan error, tentativas)
	var grupo sync.WaitGroup
	for i := range tentativas {
		grupo.Go(func() {
			erros <- repo.RegisterDCRClient(context.Background(), registroDCR(fmt.Sprintf("dcr-%d", i), instanteOAuth, teto))
		})
	}
	grupo.Wait()
	close(erros)

	recusas := 0
	for err := range erros {
		var limite *oauth.PendingClientLimitError
		if errors.As(err, &limite) {
			recusas++
			continue
		}
		require.NoError(t, err)
	}
	assert.Equal(t, tentativas-teto, recusas)
	assert.Equal(t, teto, countRows(t, pool, "SELECT count(*) FROM oauth_clients WHERE id LIKE 'dcr-%'"))
}

// segurarAuditoria abre uma transação que trava a tabela de auditoria contra gravações,
// para a aprovação parar no fim, com a Autorização já gravada e ainda não commitada.
func segurarAuditoria(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	_, err = tx.Exec(ctx, "LOCK TABLE audit_events IN EXCLUSIVE MODE")
	require.NoError(t, err)
	return tx
}

// TestPostgresOAuthRepo_RegisterDCRClient_EsperaAprovacaoAbertaDoPendente prova que a
// limpeza dos pendentes vencidos espera a aprovação ainda não commitada de um deles:
// depois do commit, o cliente deixa de ser pendente e a Autorização sobrevive.
func TestPostgresOAuthRepo_RegisterDCRClient_EsperaAprovacaoAbertaDoPendente(t *testing.T) {
	repo, pool := repoDCR(t)
	ctx := context.Background()
	clienteAntigo(t, pool, "dcr-velho", "dcr", instanteOAuth.Add(-prazoDosPendentes))
	pedido := pedidoOAuth("r1", oauth.ScopeNotesRead)
	pedido.ClientID = "dcr-velho"
	require.NoError(t, repo.CreateRequest(ctx, pedido))
	auditoria := segurarAuditoria(t, pool)
	aprovou := make(chan error, 1)
	go func() {
		_, err := repo.ApproveRequest(ctx, aprovacao("r1", "h1", instanteOAuth.Add(-time.Second)))
		aprovou <- err
	}()
	esperarBloqueios(t, pool, 1, "a aprovação deveria parar na auditoria, com a Autorização gravada e aberta")
	registrou := make(chan error, 1)
	go func() { registrou <- repo.RegisterDCRClient(ctx, registroDCR("dcr-novo", instanteOAuth, 10)) }()
	esperarBloqueios(t, pool, 2, "o registro deveria esperar a aprovação aberta")

	require.NoError(t, auditoria.Commit(ctx))

	require.NoError(t, <-aprovou)
	require.NoError(t, <-registrou)
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM oauth_authorizations WHERE client_id = 'dcr-velho'"), "a Autorização sobrevive")
	assert.Equal(t, 1, clientesComID(t, pool, "dcr-velho"))
}
