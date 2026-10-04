package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

const segredoTeste = "chave-secreta-para-testes-unitarios-de-tokens"

func TestTokenCodec_GenerateAndValidate_Sucesso(t *testing.T) {
	clock := func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC) }
	codec := auth.NewTokenCodec(segredoTeste, 15*time.Minute, clock)

	pair, err := codec.GenerateTokenPair("user-123", "tenant-456", "admin")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, int64(900), pair.ExpiresIn)

	claims, err := codec.ValidateToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.Subject)
	assert.Equal(t, "tenant-456", claims.TenantID)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, clock().Unix(), claims.IssuedAt)
	assert.Equal(t, clock().Add(15*time.Minute).Unix(), claims.ExpiresAt)
}

func TestTokenCodec_ValidateToken_TokenExpirado(t *testing.T) {
	current := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	codec := auth.NewTokenCodec(segredoTeste, 5*time.Minute, func() time.Time { return current })

	pair, err := codec.GenerateTokenPair("user-1", "tenant-1", "member")
	require.NoError(t, err)

	codecFuturo := auth.NewTokenCodec(segredoTeste, 5*time.Minute, func() time.Time {
		return current.Add(10 * time.Minute)
	})
	_, err = codecFuturo.ValidateToken(pair.AccessToken)
	require.ErrorIs(t, err, auth.ErrExpiredToken)
}

func TestTokenCodec_ValidateToken_AssinaturaInvalida(t *testing.T) {
	codec := auth.NewTokenCodec(segredoTeste, 15*time.Minute, time.Now)
	pair, err := codec.GenerateTokenPair("user-1", "tenant-1", "member")
	require.NoError(t, err)

	codecOutroSegredo := auth.NewTokenCodec("outro-segredo-completamente-diferente", 15*time.Minute, time.Now)
	_, err = codecOutroSegredo.ValidateToken(pair.AccessToken)
	require.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestTokenCodec_ValidateToken_FormatoInvalido(t *testing.T) {
	codec := auth.NewTokenCodec(segredoTeste, 15*time.Minute, time.Now)

	formatosInvalidos := []string{
		"",
		"invalido",
		"a.b",
		"a.b.c.d",
		"header.payload.assinaturaIncorreta!",
	}

	for _, token := range formatosInvalidos {
		_, err := codec.ValidateToken(token)
		require.Error(t, err, "token %q deveria falhar", token)
		assert.ErrorIs(t, err, auth.ErrInvalidToken)
	}
}

func TestHashRefreshToken_Consistencia(t *testing.T) {
	token := "token-aleatorio-para-teste"
	hash1 := auth.HashRefreshToken(token)
	hash2 := auth.HashRefreshToken(token)

	assert.NotEmpty(t, hash1)
	assert.Equal(t, hash1, hash2)
	assert.NotEqual(t, token, hash1)
}

func TestTokenCodec_ValidateToken_HeaderAlterado(t *testing.T) {
	codec := auth.NewTokenCodec(segredoTeste, 15*time.Minute, time.Now)
	pair, err := codec.GenerateTokenPair("user-1", "tenant-1", "member")
	require.NoError(t, err)

	partes := strings.Split(pair.AccessToken, ".")
	adulterado := "eyJhbGciOiJub25lIn0." + partes[1] + "." + partes[2]

	_, err = codec.ValidateToken(adulterado)
	require.ErrorIs(t, err, auth.ErrInvalidToken)
}
