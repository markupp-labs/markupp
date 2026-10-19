package storage_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

const prazoDosPendentes = 24 * time.Hour

// repoDCR sobe o banco do repoOAuth com o cliente c1 já autorizado (e revogado), para
// ele não contar como pendente.
func repoDCR(t *testing.T) (*storage.PostgresOAuthRepository, *pgxpool.Pool) {
	t.Helper()
	repo, pool := repoOAuth(t)
	require.NoError(t, autorizar(pool, "a1", "{notas:leitura}", true))
	return repo, pool
}

func registroDCR(id string, emitidoEm time.Time, teto int) oauth.DCRRegistration {
	return oauth.DCRRegistration{
		Client:   oauth.Client{ID: id, Name: "Agente " + id, RedirectURIs: []string{"http://127.0.0.1/cb", "meuapp://cb"}},
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

	got, err := repo.GetClient(context.Background(), "dcr-1")
	require.NoError(t, err)
	assert.Equal(t, reg.Client, got)
	assert.Equal(t, 1, countRows(t, pool, `SELECT count(*) FROM oauth_clients WHERE id = 'dcr-1'
		AND source = 'dcr' AND metadata_expires_at IS NULL AND created_at = '2026-10-16T12:00:00Z'`))
}

func TestPostgresOAuthRepo_RegisterDCRClient_ApagaSoOsPendentesVencidos(t *testing.T) {
	repo, pool := repoDCR(t)
	limite := instanteOAuth.Add(-prazoDosPendentes)
	clienteAntigo(t, pool, "dcr-vencido", "dcr", limite)
	clienteAntigo(t, pool, "dcr-quase", "dcr", limite.Add(time.Second))
	clienteAntigo(t, pool, "dcr-autorizado", "dcr", limite.Add(-time.Hour))
	clienteAntigo(t, pool, "cimd-antigo", "cimd", limite.Add(-time.Hour))
	_, err := pool.Exec(context.Background(), `INSERT INTO oauth_authorizations (id, user_id, client_id, scopes, revoked_at)
		VALUES ('a-antiga', 'u1', 'dcr-autorizado', '{notas:leitura}', now())`)
	require.NoError(t, err)

	require.NoError(t, repo.RegisterDCRClient(context.Background(), registroDCR("dcr-novo", instanteOAuth, 10)))

	assert.Zero(t, clientesComID(t, pool, "dcr-vencido"), "pendente com o prazo cumprido sai")
	for _, id := range []string{"dcr-quase", "dcr-autorizado", "cimd-antigo", "c1", "dcr-novo"} {
		assert.Equal(t, 1, clientesComID(t, pool, id), id)
	}
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
