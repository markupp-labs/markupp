package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

func TestInstanceRegistration_BancoNovo_SemEscolha(t *testing.T) {
	repo := storage.NewInstanceRegistration(bancoDaInstancia(t))

	choices, err := repo.LoadRegistrationChoices(context.Background())

	require.NoError(t, err)
	assert.Empty(t, choices)
}

func TestInstanceRegistration_AdminGrava_CadastroSemAtorLe(t *testing.T) {
	repo := storage.NewInstanceRegistration(bancoDaInstancia(t))

	require.NoError(t, repo.SaveRegistrationChoice(comAtor(), "local", false))
	require.NoError(t, repo.SaveRegistrationChoice(comAtor(), "github", true))
	choices, err := repo.LoadRegistrationChoices(context.Background())

	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"local": false, "github": true}, choices)
}

func TestInstanceRegistration_GravarDeNovo_TrocaOValor(t *testing.T) {
	repo := storage.NewInstanceRegistration(bancoDaInstancia(t))
	require.NoError(t, repo.SaveRegistrationChoice(comAtor(), "local", false))

	require.NoError(t, repo.SaveRegistrationChoice(comAtor(), "local", true))
	choices, err := repo.LoadRegistrationChoices(context.Background())

	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"local": true}, choices)
}

func TestInstanceRegistration_GravarSemAtor_Recusa(t *testing.T) {
	repo := storage.NewInstanceRegistration(bancoDaInstancia(t))

	err := repo.SaveRegistrationChoice(context.Background(), "local", false)

	require.ErrorIs(t, err, storage.ErrNoActor)
}

func TestInstanceRegistration_BancoNovo_SemConta(t *testing.T) {
	repo := storage.NewInstanceRegistration(bancoDaInstancia(t))

	has, err := repo.HasAccounts(context.Background())

	require.NoError(t, err)
	assert.False(t, has)
}
