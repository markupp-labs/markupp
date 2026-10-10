package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func TestBcryptHasher_HashECompare_Sucesso(t *testing.T) {
	hasher := auth.NewBcryptHasher()
	senha := "minhaSenhaSuperSegura123"

	hash, err := hasher.Hash(senha)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, senha, hash)

	err = hasher.Compare(hash, senha)
	require.NoError(t, err)
}

func TestBcryptHasher_CompareSenhaIncorreta_RetornaErro(t *testing.T) {
	hasher := auth.NewBcryptHasher()
	senha := "senhaCorreta"

	hash, err := hasher.Hash(senha)
	require.NoError(t, err)

	err = hasher.Compare(hash, "senhaIncorreta")
	require.Error(t, err)
}
