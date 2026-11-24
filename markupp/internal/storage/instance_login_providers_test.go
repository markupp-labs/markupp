package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

func provedorDoGoogle() instance.LoginProviderRecord {
	return instance.LoginProviderRecord{ID: "google", Name: "Google", Issuer: "https://accounts.google.com",
		ClientID: "id-google", SealedSecret: "selado-google", Enabled: true}
}

func TestInstanceLoginProviders_BancoNovo_SemProvedor(t *testing.T) {
	repo := storage.NewInstanceLoginProviders(bancoDaInstancia(t))

	records, err := repo.ListLoginProviders(context.Background())

	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestInstanceLoginProviders_AdminCria_LoginSemAtorLe(t *testing.T) {
	repo := storage.NewInstanceLoginProviders(bancoDaInstancia(t))

	require.NoError(t, repo.CreateLoginProvider(comAtor(), provedorDoGoogle()))
	require.NoError(t, repo.CreateLoginProvider(comAtor(), instance.LoginProviderRecord{ID: "github", Name: "GitHub", ClientID: "id-github", SealedSecret: "selado-github"})) //nolint:gosec // G101: segredo de teste
	records, err := repo.ListLoginProviders(context.Background())

	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "google", records[0].ID, "na ordem de criação")
	assert.Equal(t, "https://accounts.google.com", records[0].Issuer)
	assert.Equal(t, "selado-google", records[0].SealedSecret)
	assert.True(t, records[0].Enabled)
	assert.False(t, records[1].Enabled)
	assert.False(t, records[0].UpdatedAt.IsZero())
}

func TestInstanceLoginProviders_CriarIdRepetido_Recusa(t *testing.T) {
	repo := storage.NewInstanceLoginProviders(bancoDaInstancia(t))
	require.NoError(t, repo.CreateLoginProvider(comAtor(), provedorDoGoogle()))

	err := repo.CreateLoginProvider(comAtor(), provedorDoGoogle())

	require.ErrorIs(t, err, instance.ErrLoginProviderExists)
}

func TestInstanceLoginProviders_CriarSemAtor_Recusa(t *testing.T) {
	repo := storage.NewInstanceLoginProviders(bancoDaInstancia(t))

	err := repo.CreateLoginProvider(context.Background(), provedorDoGoogle())

	require.ErrorIs(t, err, storage.ErrNoActor)
}

func TestInstanceLoginProviders_Atualizar_TrocaOsCamposEARevisao(t *testing.T) {
	repo := storage.NewInstanceLoginProviders(bancoDaInstancia(t))
	require.NoError(t, repo.CreateLoginProvider(comAtor(), provedorDoGoogle()))
	before, err := repo.ListLoginProviders(context.Background())
	require.NoError(t, err)

	changed := provedorDoGoogle()
	changed.Name, changed.ClientID, changed.SealedSecret, changed.Enabled = "Google Workspace", "id-novo", "selado-novo", false
	require.NoError(t, repo.UpdateLoginProvider(comAtor(), changed))
	records, err := repo.ListLoginProviders(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "Google Workspace", records[0].Name)
	assert.Equal(t, "id-novo", records[0].ClientID)
	assert.Equal(t, "selado-novo", records[0].SealedSecret)
	assert.False(t, records[0].Enabled)
	assert.True(t, records[0].UpdatedAt.After(before[0].UpdatedAt))
}

func TestInstanceLoginProviders_AtualizarInexistente_DizQueNaoAchou(t *testing.T) {
	repo := storage.NewInstanceLoginProviders(bancoDaInstancia(t))

	err := repo.UpdateLoginProvider(comAtor(), provedorDoGoogle())

	require.ErrorIs(t, err, instance.ErrLoginProviderNotFound)
}

func TestInstanceLoginProviders_ContaVinculada_AparecaNaContagem(t *testing.T) {
	pool := bancoDaInstancia(t)
	repo := storage.NewInstanceLoginProviders(pool)
	require.NoError(t, repo.CreateLoginProvider(comAtor(), provedorDoGoogle()))
	vinculaContaAoProvedor(t, pool, "google", "sub-1")

	records, err := repo.ListLoginProviders(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, records[0].LinkedAccounts)
}

// Apagar leva os vínculos junto: um provedor novo com o mesmo id não religa contas antigas (ADR-0060).
func TestInstanceLoginProviders_Apagar_LevaOsVinculosEAEscolhaDoCadastro(t *testing.T) {
	pool := bancoDaInstancia(t)
	repo := storage.NewInstanceLoginProviders(pool)
	require.NoError(t, repo.CreateLoginProvider(comAtor(), provedorDoGoogle()))
	vinculaContaAoProvedor(t, pool, "google", "sub-1")
	require.NoError(t, storage.NewInstanceRegistration(pool).SaveRegistrationChoice(comAtor(), "google", false))

	removed, err := repo.DeleteLoginProvider(comAtor(), "google")

	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	records, err := repo.ListLoginProviders(context.Background())
	require.NoError(t, err)
	assert.Empty(t, records)
	assert.Zero(t, contaVinculosDoProvedor(t, pool, "google"))
	choices, err := storage.NewInstanceRegistration(pool).LoadRegistrationChoices(context.Background())
	require.NoError(t, err)
	assert.NotContains(t, choices, "google")
}

func TestInstanceLoginProviders_ApagarInexistente_DizQueNaoAchou(t *testing.T) {
	repo := storage.NewInstanceLoginProviders(bancoDaInstancia(t))

	_, err := repo.DeleteLoginProvider(comAtor(), "google")

	require.ErrorIs(t, err, instance.ErrLoginProviderNotFound)
}

func TestInstanceLoginProviders_NovoSelo_TrocaSoOSegredoLido(t *testing.T) {
	repo := storage.NewInstanceLoginProviders(bancoDaInstancia(t))
	require.NoError(t, repo.CreateLoginProvider(comAtor(), provedorDoGoogle()))
	before, err := repo.ListLoginProviders(context.Background())
	require.NoError(t, err)

	require.NoError(t, repo.ResealLoginProviderSecret(context.Background(), "google", "selado-google", "selo-novo"))
	require.NoError(t, repo.ResealLoginProviderSecret(context.Background(), "google", "selado-velho", "outro"))
	records, err := repo.ListLoginProviders(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "selo-novo", records[0].SealedSecret)
	assert.Equal(t, before[0].UpdatedAt, records[0].UpdatedAt, "o selo novo não muda a revisão")
}

// vinculaContaAoProvedor cria uma conta com a identidade subject no provedor.
func vinculaContaAoProvedor(t *testing.T, pool *pgxpool.Pool, provider, subject string) {
	t.Helper()
	userID := "u-" + subject
	_, err := pool.Exec(context.Background(), `INSERT INTO users (id, email, created_at, updated_at) VALUES ($1, $2, now(), now())`, userID, userID+"@markupp.dev")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `INSERT INTO user_identities (id, user_id, provider, provider_user_id, created_at) VALUES ($1, $2, $3, $4, now())`,
		"i-"+subject, userID, provider, subject)
	require.NoError(t, err)
}

func contaVinculosDoProvedor(t *testing.T, pool *pgxpool.Pool, provider string) int {
	t.Helper()
	var count int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM user_identities WHERE provider = $1`, provider).Scan(&count))
	return count
}
