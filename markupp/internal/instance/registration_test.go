package instance_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
)

// FakeRegistrationStore guarda a escolha por método em memória; noAccounts é a instalação nova.
type FakeRegistrationStore struct {
	choices    map[string]bool
	noAccounts bool
}

func (f *FakeRegistrationStore) HasAccounts(context.Context) (bool, error) {
	return !f.noAccounts, nil
}

func (f *FakeRegistrationStore) LoadRegistrationChoices(context.Context) (map[string]bool, error) {
	return f.choices, nil
}

func (f *FakeRegistrationStore) SaveRegistrationChoice(_ context.Context, method string, open bool) error {
	f.choices[method] = open
	return nil
}

func fechado() *bool { v := false; return &v }

// metodosDeTeste são senha e GitHub sem trava, e um OIDC que quem hospeda fechou.
func metodosDeTeste() []instance.RegistrationMethod {
	return []instance.RegistrationMethod{
		{ID: "local", Name: "Email e senha"},
		{ID: "github", Name: "GitHub"},
		{ID: "empresa", Name: "Empresa", HostSetting: fechado()},
	}
}

func cadastro(enabled bool, choices map[string]bool) (*instance.RegistrationService, *FakeRegistrationStore) {
	store := &FakeRegistrationStore{choices: choices}
	return instance.NewRegistrationService(store, enabled, instance.RegistrationMethodList(metodosDeTeste())), store
}

func TestRegistrationMethods_SemEscolha_AbertosMenosOTravado(t *testing.T) {
	svc, _ := cadastro(true, map[string]bool{})

	views, err := svc.Methods(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []instance.RegistrationMethodView{
		{ID: "local", Name: "Email e senha", Open: true},
		{ID: "github", Name: "GitHub", Open: true},
		{ID: "empresa", Name: "Empresa", Open: false, Locked: true},
	}, views)
}

func TestRegistrationMethods_ComEscolha_MostraAEscolha(t *testing.T) {
	svc, _ := cadastro(true, map[string]bool{"local": false, "empresa": true})

	views, err := svc.Methods(context.Background())

	require.NoError(t, err)
	assert.False(t, views[0].Open)
	assert.False(t, views[2].Open, "a variável de quem hospeda vence a escolha")
}

func TestRegistrationMethods_CadastroGeralDesligado_TodosFechadosETravados(t *testing.T) {
	svc, _ := cadastro(false, map[string]bool{"github": true})

	views, err := svc.Methods(context.Background())

	require.NoError(t, err)
	for _, view := range views {
		assert.False(t, view.Open, view.ID)
		assert.True(t, view.Locked, view.ID)
	}
}

func TestSetRegistrationOpen_MetodoLivre_GravaEDevolve(t *testing.T) {
	svc, store := cadastro(true, map[string]bool{})

	view, err := svc.SetOpen(context.Background(), "local", false)

	require.NoError(t, err)
	assert.Equal(t, instance.RegistrationMethodView{ID: "local", Name: "Email e senha", Open: false}, view)
	assert.Equal(t, map[string]bool{"local": false}, store.choices)
}

func TestSetRegistrationOpen_MetodoTravado_Recusa(t *testing.T) {
	svc, store := cadastro(true, map[string]bool{})

	_, err := svc.SetOpen(context.Background(), "empresa", true)

	require.ErrorIs(t, err, instance.ErrRegistrationLocked)
	assert.Empty(t, store.choices)
}

func TestSetRegistrationOpen_CadastroGeralDesligado_Recusa(t *testing.T) {
	svc, _ := cadastro(false, map[string]bool{})

	_, err := svc.SetOpen(context.Background(), "local", true)

	require.ErrorIs(t, err, instance.ErrRegistrationLocked)
}

func TestSetRegistrationOpen_MetodoDesconhecido_Recusa(t *testing.T) {
	svc, _ := cadastro(true, map[string]bool{})

	_, err := svc.SetOpen(context.Background(), "gitlab", true)

	require.ErrorIs(t, err, instance.ErrUnknownRegistrationMethod)
	assert.ErrorContains(t, err, "gitlab")
}

func TestRegistrationOpen_SegueOQueOMetodoMostra(t *testing.T) {
	svc, _ := cadastro(true, map[string]bool{"github": false})

	for method, want := range map[string]bool{"local": true, "github": false, "empresa": false, "gitlab": false} {
		open, err := svc.RegistrationOpen(context.Background(), method)

		require.NoError(t, err)
		assert.Equal(t, want, open, method)
	}
}

func TestRegistrationOpen_CadastroGeralDesligado_Fechado(t *testing.T) {
	svc, _ := cadastro(false, map[string]bool{"local": true})

	open, err := svc.RegistrationOpen(context.Background(), "local")

	require.NoError(t, err)
	assert.False(t, open)
}

func TestRegistrationEnabled_SegueOInterruptorGeral(t *testing.T) {
	ligado, _ := cadastro(true, map[string]bool{})
	desligado, _ := cadastro(false, map[string]bool{})

	assert.True(t, ligado.RegistrationEnabled())
	assert.False(t, desligado.RegistrationEnabled())
}

func TestRegistrationOpen_InstalacaoSemConta_AbertoMesmoComTudoFechado(t *testing.T) {
	svc, store := cadastro(false, map[string]bool{"local": false})
	store.noAccounts = true

	for method, want := range map[string]bool{"local": true, "empresa": true, "gitlab": false} {
		open, err := svc.RegistrationOpen(context.Background(), method)

		require.NoError(t, err)
		assert.Equal(t, want, open, method)
	}
}
