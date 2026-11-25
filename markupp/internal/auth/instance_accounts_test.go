package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// fakeInstanceAccountsRepository guarda o que o serviço pede e devolve o que o teste pôs.
type fakeInstanceAccountsRepository struct {
	accounts map[string]auth.AccountDetail
	query    auth.AccountQuery

	blockedWith auth.AuditEventRecord
	blockedTo   *bool
	deletedWith auth.AuditEventRecord
}

func (f *fakeInstanceAccountsRepository) ListAccounts(_ context.Context, q auth.AccountQuery) ([]auth.AccountSummary, error) {
	f.query = q
	var list []auth.AccountSummary
	for _, a := range f.accounts {
		list = append(list, a.AccountSummary)
	}
	return list, nil
}

func (f *fakeInstanceAccountsRepository) GetAccountDetail(_ context.Context, id string) (auth.AccountDetail, error) {
	a, ok := f.accounts[id]
	if !ok {
		return auth.AccountDetail{}, auth.ErrUserNotFound
	}
	return a, nil
}

func (f *fakeInstanceAccountsRepository) SetAccountBlocked(_ context.Context, id string, blocked bool, event auth.AuditEventRecord) error {
	if _, ok := f.accounts[id]; !ok {
		return auth.ErrUserNotFound
	}
	f.blockedTo, f.blockedWith = &blocked, event
	return nil
}

func (f *fakeInstanceAccountsRepository) DeleteAccount(_ context.Context, id string, event auth.AuditEventRecord) error {
	if _, ok := f.accounts[id]; !ok {
		return auth.ErrUserNotFound
	}
	f.deletedWith = event
	return nil
}

var momentoDaAdministracao = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func contasDaInstancia() (*fakeInstanceAccountsRepository, *auth.InstanceAccounts) {
	repo := &fakeInstanceAccountsRepository{accounts: map[string]auth.AccountDetail{
		"u-ana": {AccountSummary: auth.AccountSummary{ID: "u-ana", Email: "ana@markupp.dev"}},
	}}
	ids := 0
	svc := auth.NewInstanceAccounts(repo, func() time.Time { return momentoDaAdministracao }, func() string {
		ids++
		return "evento-" + string(rune('0'+ids))
	})
	return repo, svc
}

func TestInstanceAccounts_Listar_PadraoDe50(t *testing.T) {
	repo, svc := contasDaInstancia()

	_, err := svc.List(context.Background(), auth.AccountQuery{Email: "ana"})

	require.NoError(t, err)
	assert.Equal(t, auth.AccountQuery{Email: "ana", Limit: 50}, repo.query)
}

func TestInstanceAccounts_Listar_StatusDesconhecido_Recusa(t *testing.T) {
	_, svc := contasDaInstancia()

	_, err := svc.List(context.Background(), auth.AccountQuery{Status: "suspensa"})

	require.ErrorIs(t, err, auth.ErrInvalidAccountQuery)
	assert.Contains(t, err.Error(), `"suspensa"`)
}

func TestInstanceAccounts_Bloquear_AuditaQuemBloqueou(t *testing.T) {
	repo, svc := contasDaInstancia()

	require.NoError(t, svc.SetBlocked(context.Background(), "u-admin", "u-ana", true))

	require.NotNil(t, repo.blockedTo)
	assert.True(t, *repo.blockedTo)
	assert.Equal(t, auth.AuditEventRecord{ID: "evento-1", ActorID: "u-admin", Action: "accounts.block",
		Target: "u-ana", Result: "success", OccurredAt: momentoDaAdministracao}, repo.blockedWith)
}

func TestInstanceAccounts_Desbloquear_AuditaComoDesbloqueio(t *testing.T) {
	repo, svc := contasDaInstancia()

	require.NoError(t, svc.SetBlocked(context.Background(), "u-admin", "u-ana", false))

	assert.False(t, *repo.blockedTo)
	assert.Equal(t, "accounts.unblock", repo.blockedWith.Action)
}

func TestInstanceAccounts_BloquearOuApagarAPropria_Recusa(t *testing.T) {
	repo, svc := contasDaInstancia()

	blockErr := svc.SetBlocked(context.Background(), "u-ana", "u-ana", true)
	deleteErr := svc.Delete(context.Background(), "u-ana", "u-ana")

	require.ErrorIs(t, blockErr, auth.ErrCannotTargetSelf)
	require.ErrorIs(t, deleteErr, auth.ErrCannotTargetSelf)
	assert.Nil(t, repo.blockedTo)
	assert.Empty(t, repo.deletedWith.ID)
}

func TestInstanceAccounts_Apagar_Audita(t *testing.T) {
	repo, svc := contasDaInstancia()

	require.NoError(t, svc.Delete(context.Background(), "u-admin", "u-ana"))

	assert.Equal(t, "accounts.delete", repo.deletedWith.Action)
	assert.Equal(t, "u-admin", repo.deletedWith.ActorID)
	assert.Equal(t, "u-ana", repo.deletedWith.Target)
}

func TestInstanceAccounts_ContaInexistente_NaoAcha(t *testing.T) {
	_, svc := contasDaInstancia()

	_, getErr := svc.Get(context.Background(), "u-nenhuma")
	blockErr := svc.SetBlocked(context.Background(), "u-admin", "u-nenhuma", true)

	require.ErrorIs(t, getErr, auth.ErrUserNotFound)
	require.ErrorIs(t, blockErr, auth.ErrUserNotFound)
}
