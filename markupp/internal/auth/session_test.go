package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

var origemDoNavegador = auth.Origin{IP: "203.0.113.7", UserAgent: "Firefox/140"}

func sessaoDoToken(t *testing.T, repo *fakeRepository, rawRefresh string) auth.SessionRecord {
	t.Helper()
	rec, ok := repo.refreshTokens[auth.HashOpaqueToken(rawRefresh)]
	require.True(t, ok, "refresh %q não foi gravado", rawRefresh)
	sessao, ok := repo.sessions[rec.SessionID]
	require.True(t, ok, "sessão %q do refresh não foi gravada", rec.SessionID)
	return sessao
}

func TestService_Login_AbreSessaoComOrigem(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	ctx := auth.WithOrigin(context.Background(), origemDoNavegador)

	pair, err := svc.Register(ctx, "admin@markupp.dev", "senha-de-teste-longa")
	require.NoError(t, err)

	sessao := sessaoDoToken(t, repo, pair.RefreshToken)
	assert.NotEmpty(t, sessao.ID)
	assert.Equal(t, origemDoNavegador, sessao.Origin)
	assert.Equal(t, clock(), sessao.CreatedAt)
	assert.Equal(t, clock(), sessao.LastUsedAt)
}

func TestService_Refresh_MantemSessaoEAtualizaUltimoUso(t *testing.T) {
	agora := defaultClock()()
	clock := func() time.Time { return agora }
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	pair1, err := svc.Register(context.Background(), "admin@markupp.dev", "senha-de-teste-longa")
	require.NoError(t, err)
	criada := agora
	agora = agora.Add(time.Hour)

	pair2, err := svc.RefreshToken(auth.WithOrigin(context.Background(), origemDoNavegador), pair1.RefreshToken)
	require.NoError(t, err)

	antes := repo.refreshTokens[auth.HashOpaqueToken(pair1.RefreshToken)]
	sessao := sessaoDoToken(t, repo, pair2.RefreshToken)
	assert.Equal(t, antes.SessionID, sessao.ID, "a renovação continua a mesma sessão")
	assert.Equal(t, criada, sessao.CreatedAt)
	assert.Equal(t, agora, sessao.LastUsedAt)
	assert.Equal(t, origemDoNavegador, sessao.Origin, "a origem é a da última renovação")
	assert.Len(t, repo.sessions, 1)
}

func TestService_Refresh_TokenAntigoSemSessao_AbreSessaoNova(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)
	pair1, err := svc.Register(context.Background(), "admin@markupp.dev", "senha-de-teste-longa")
	require.NoError(t, err)
	hash := auth.HashOpaqueToken(pair1.RefreshToken)
	legado := repo.refreshTokens[hash]
	legado.SessionID = ""
	repo.refreshTokens[hash] = legado
	repo.sessions = map[string]auth.SessionRecord{}

	pair2, err := svc.RefreshToken(context.Background(), pair1.RefreshToken)
	require.NoError(t, err)

	assert.NotEmpty(t, sessaoDoToken(t, repo, pair2.RefreshToken).ID)
}

func TestService_Login_TokenDeAcessoLevaASessao(t *testing.T) {
	clock := defaultClock()
	repo := newFakeRepository(clock)
	svc := setupAuthService(repo, false, clock)

	pair, err := svc.Register(context.Background(), "admin@markupp.dev", "senha-de-teste-longa")
	require.NoError(t, err)

	claims, err := auth.NewTokenCodec("segredo-para-testes-de-servico", 15*time.Minute, clock).ValidateToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, sessaoDoToken(t, repo, pair.RefreshToken).ID, claims.SessionID, "o claim sid do OIDC")
}
