package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

const clienteCIMD = "https://agente.example/oauth/client.json"

func documentoCIMD(nome string, redirects ...string) oauth.Client {
	return oauth.Client{ID: clienteCIMD, Name: nome, RedirectURIs: redirects}
}

func TestPostgresOAuthRepo_SaveCIMDClient_CriaCliente(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	cliente := documentoCIMD("Agente CIMD", "https://agente.example/cb")

	require.NoError(t, repo.SaveCIMDClient(ctx, cliente, instanteOAuth.Add(time.Hour), instanteOAuth))

	got, err := repo.GetFreshCIMDClient(ctx, clienteCIMD, instanteOAuth)
	require.NoError(t, err)
	assert.Equal(t, cliente, got)
}

func TestPostgresOAuthRepo_SaveCIMDClient_AtualizaDocumento(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	require.NoError(t, repo.SaveCIMDClient(ctx, documentoCIMD("Antigo", "https://agente.example/cb"), instanteOAuth.Add(time.Minute), instanteOAuth))
	novo := documentoCIMD("Novo", "https://agente.example/cb2", "http://127.0.0.1/cb")

	require.NoError(t, repo.SaveCIMDClient(ctx, novo, instanteOAuth.Add(2*time.Hour), instanteOAuth))

	got, err := repo.GetFreshCIMDClient(ctx, clienteCIMD, instanteOAuth.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, novo, got)
}

func TestPostgresOAuthRepo_SaveCIMDClient_NaoSobrescreveClienteDCR(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	intruso := oauth.Client{ID: "c1", Name: "Intruso", RedirectURIs: []string{"https://intruso.example/cb"}}

	err := repo.SaveCIMDClient(ctx, intruso, instanteOAuth.Add(time.Hour), instanteOAuth)

	require.ErrorIs(t, err, oauth.ErrClientSourceConflict)
	assert.ErrorContains(t, err, `esperado source "cimd"`)
	got, err := repo.GetClient(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, oauth.Client{ID: "c1", Name: "Agente", RedirectURIs: []string{"http://127.0.0.1/cb"}}, got)
	assert.Equal(t, 1, countRows(t, pool, `SELECT count(*) FROM oauth_clients
		WHERE id = 'c1' AND source = 'dcr' AND metadata_expires_at IS NULL`))
}

func TestPostgresOAuthRepo_GetFreshCIMDClient_IgnoraVencido(t *testing.T) {
	repo, _ := repoOAuth(t)
	ctx := context.Background()
	expira := instanteOAuth.Add(time.Hour)
	require.NoError(t, repo.SaveCIMDClient(ctx, documentoCIMD("Agente CIMD", "https://agente.example/cb"), expira, instanteOAuth))

	_, err := repo.GetFreshCIMDClient(ctx, clienteCIMD, expira)

	require.ErrorIs(t, err, oauth.ErrClientNotFound)
}

// clienteCIMDDe é um cliente CIMD de id próprio, para os testes com vários clientes.
func clienteCIMDDe(id string) oauth.Client {
	return oauth.Client{ID: id, Name: "Agente " + id, RedirectURIs: []string{"https://agente.example/cb"}}
}

func clientesComID(t *testing.T, pool *pgxpool.Pool, id string) int {
	t.Helper()
	return countRows(t, pool, `SELECT count(*) FROM oauth_clients WHERE id = '`+id+`'`)
}

func TestPostgresOAuthRepo_SaveCIMDClient_ApagaVencidosSemAutorizacaoAtiva(t *testing.T) {
	repo, pool := repoOAuth(t)
	ctx := context.Background()
	gravacao := instanteOAuth.Add(time.Hour)
	vencido, revogado, ativo, fresco := "https://vencido.example/c.json", "https://revogado.example/c.json",
		"https://ativo.example/c.json", "https://fresco.example/c.json"
	require.NoError(t, repo.SaveCIMDClient(ctx, clienteCIMDDe(vencido), gravacao, instanteOAuth))
	require.NoError(t, repo.SaveCIMDClient(ctx, clienteCIMDDe(revogado), instanteOAuth.Add(time.Minute), instanteOAuth))
	require.NoError(t, repo.SaveCIMDClient(ctx, clienteCIMDDe(ativo), instanteOAuth.Add(time.Minute), instanteOAuth))
	require.NoError(t, repo.SaveCIMDClient(ctx, clienteCIMDDe(fresco), gravacao.Add(time.Second), instanteOAuth))
	_, err := pool.Exec(ctx, `INSERT INTO oauth_authorizations (id, user_id, client_id, scopes, revoked_at)
		VALUES ('a-revogada', 'u1', $1, ARRAY['notas:leitura'], $2), ('a-ativa', 'u1', $3, ARRAY['notas:leitura'], NULL)`,
		revogado, instanteOAuth, ativo)
	require.NoError(t, err)

	require.NoError(t, repo.SaveCIMDClient(ctx, documentoCIMD("Agente CIMD", "https://agente.example/cb"), gravacao.Add(time.Hour), gravacao))

	assert.Zero(t, clientesComID(t, pool, vencido), "vencido no instante da gravação e sem Autorização")
	assert.Zero(t, clientesComID(t, pool, revogado), "só com Autorização revogada, como no DCR")
	assert.Equal(t, 1, clientesComID(t, pool, ativo), "com Autorização ativa")
	assert.Equal(t, 1, clientesComID(t, pool, fresco))
	assert.Equal(t, 1, clientesComID(t, pool, clienteCIMD), "o recém-gravado fica")
	assert.Equal(t, 1, clientesComID(t, pool, "c1"), "o cliente DCR não é tocado")
}
