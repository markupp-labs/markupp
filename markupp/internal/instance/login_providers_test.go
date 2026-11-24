package instance_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

// FakeLoginProviderStore guarda os provedores em memória, na ordem de criação.
type FakeLoginProviderStore struct {
	records  []instance.LoginProviderRecord
	resealed map[string]string
	clock    time.Time
	err      error
}

func (f *FakeLoginProviderStore) ListLoginProviders(context.Context) ([]instance.LoginProviderRecord, error) {
	return append([]instance.LoginProviderRecord(nil), f.records...), f.err
}

func (f *FakeLoginProviderStore) CreateLoginProvider(_ context.Context, r instance.LoginProviderRecord) error {
	if _, ok := f.find(r.ID); ok {
		return instance.ErrLoginProviderExists
	}
	f.clock = f.clock.Add(time.Second)
	r.UpdatedAt = f.clock
	f.records = append(f.records, r)
	return nil
}

func (f *FakeLoginProviderStore) UpdateLoginProvider(_ context.Context, r instance.LoginProviderRecord) error {
	i, ok := f.find(r.ID)
	if !ok {
		return instance.ErrLoginProviderNotFound
	}
	f.clock = f.clock.Add(time.Second)
	r.UpdatedAt, r.LinkedAccounts = f.clock, f.records[i].LinkedAccounts
	f.records[i] = r
	return nil
}

func (f *FakeLoginProviderStore) DeleteLoginProvider(_ context.Context, id string) (int, error) {
	i, ok := f.find(id)
	if !ok {
		return 0, instance.ErrLoginProviderNotFound
	}
	links := f.records[i].LinkedAccounts
	f.records = append(f.records[:i], f.records[i+1:]...)
	return links, nil
}

func (f *FakeLoginProviderStore) ResealLoginProviderSecret(_ context.Context, id, _, fresh string) error {
	if f.resealed == nil {
		f.resealed = map[string]string{}
	}
	f.resealed[id] = fresh
	return nil
}

func (f *FakeLoginProviderStore) find(id string) (int, bool) {
	for i, r := range f.records {
		if r.ID == id {
			return i, true
		}
	}
	return 0, false
}

func callbackDeTeste(id string) string { return "https://painel.exemplo/auth/" + id + "/callback" }

func servicoDeLogin(records ...instance.LoginProviderRecord) (*instance.LoginProviderService, *FakeLoginProviderStore) {
	store := &FakeLoginProviderStore{records: records}
	return instance.NewLoginProviderService(store, FakeSealer{}, nil, callbackDeTeste), store
}

func segredo(s string) *string { return &s }

func google() instance.LoginProviderInput {
	return instance.LoginProviderInput{ID: "google", Name: "Google", Issuer: "https://accounts.google.com", ClientID: "id-google", ClientSecret: segredo("s-google"), Enabled: true}
}

func TestLoginProviders_Criar_SelaOSegredoEMostraSoQueEleExiste(t *testing.T) {
	svc, store := servicoDeLogin()

	view, err := svc.Create(context.Background(), google())

	require.NoError(t, err)
	assert.Equal(t, "selado:s-google", store.records[0].SealedSecret)
	assert.True(t, view.SecretSet)
	assert.Equal(t, "https://painel.exemplo/auth/google/callback", view.CallbackURL)
	assert.Equal(t, "id-google", view.ClientID)
}

func TestLoginProviders_CriarSemNome_UsaOId(t *testing.T) {
	svc, _ := servicoDeLogin()
	input := google()
	input.Name = ""

	view, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, "google", view.Name)
}

func TestLoginProviders_CriarInvalido_RecusaComOCampo(t *testing.T) {
	casos := map[string]func(*instance.LoginProviderInput){
		"id":            func(i *instance.LoginProviderInput) { i.ID = "Google" },
		"local":         func(i *instance.LoginProviderInput) { i.ID = "local" },
		"issuer":        func(i *instance.LoginProviderInput) { i.Issuer = "http://accounts.google.com" },
		"client_id":     func(i *instance.LoginProviderInput) { i.ClientID = " " },
		"client_secret": func(i *instance.LoginProviderInput) { i.ClientSecret = nil },
	}
	for campo, muda := range casos {
		t.Run(campo, func(t *testing.T) {
			svc, store := servicoDeLogin()
			input := google()
			muda(&input)

			_, err := svc.Create(context.Background(), input)

			require.ErrorIs(t, err, instance.ErrInvalidLoginProvider)
			assert.Empty(t, store.records)
		})
	}
}

func TestLoginProviders_GitHubSemIssuer_Aceita(t *testing.T) {
	svc, _ := servicoDeLogin()

	_, err := svc.Create(context.Background(), instance.LoginProviderInput{ID: "github", ClientID: "id", ClientSecret: segredo("s"), Enabled: true})

	require.NoError(t, err)
}

func TestLoginProviders_AtualizarSemSegredo_MantemOGuardado(t *testing.T) {
	svc, store := servicoDeLogin()
	_, err := svc.Create(context.Background(), google())
	require.NoError(t, err)
	input := google()
	input.ClientSecret, input.Name = nil, "Google Workspace"

	_, err = svc.Update(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, "selado:s-google", store.records[0].SealedSecret)
	assert.Equal(t, "Google Workspace", store.records[0].Name)
}

// O mesmo id de pessoa em outro issuer pode ser outra pessoa (ADR-0060).
func TestLoginProviders_TrocarIssuerComContaVinculada_Recusa(t *testing.T) {
	svc, _ := servicoDeLogin(instance.LoginProviderRecord{ID: "google", Name: "Google", Issuer: "https://accounts.google.com",
		ClientID: "id", SealedSecret: "selado:s", Enabled: true, LinkedAccounts: 2})
	input := google()
	input.Issuer = "https://outro.exemplo"

	_, err := svc.Update(context.Background(), input)

	require.ErrorIs(t, err, instance.ErrLoginProviderIssuerLocked)
}

func TestLoginProviders_TrocarIssuerSemContaVinculada_Aceita(t *testing.T) {
	svc, store := servicoDeLogin(instance.LoginProviderRecord{ID: "google", Name: "Google", Issuer: "https://accounts.google.com",
		ClientID: "id", SealedSecret: "selado:s", Enabled: true})
	input := google()
	input.Issuer = "https://outro.exemplo"

	_, err := svc.Update(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, "https://outro.exemplo", store.records[0].Issuer)
}

func TestLoginProviders_AtualizarInexistente_DizQueNaoAchou(t *testing.T) {
	svc, _ := servicoDeLogin()

	_, err := svc.Update(context.Background(), google())

	require.ErrorIs(t, err, instance.ErrLoginProviderNotFound)
}

func TestLoginProviders_Apagar_DizQuantosVinculosSairam(t *testing.T) {
	svc, store := servicoDeLogin(instance.LoginProviderRecord{ID: "google", SealedSecret: "selado:s", LinkedAccounts: 3})

	removed, err := svc.Delete(context.Background(), "google")

	require.NoError(t, err)
	assert.Equal(t, 3, removed)
	assert.Empty(t, store.records)
}

func TestLoginProviders_Listar_SeloVelhoGanhaSeloNovo(t *testing.T) {
	svc, store := servicoDeLogin(instance.LoginProviderRecord{ID: "google", Name: "Google", Issuer: "https://accounts.google.com",
		ClientID: "id", SealedSecret: "velho:s", Enabled: true, LinkedAccounts: 1})

	view, err := svc.List(context.Background())

	require.NoError(t, err)
	assert.False(t, view.Locked)
	require.Len(t, view.Providers, 1)
	assert.Equal(t, 1, view.Providers[0].LinkedAccounts)
	assert.Equal(t, "selado:s", store.resealed["google"])
}

func TestLoginProviders_ComProvedoresDeQuemHospeda_ListaTravadaComOsDele(t *testing.T) {
	host := []loginprovider.Settings{{ID: "github", Name: "GitHub", ClientID: "id-host", ClientSecret: "s"}}
	svc := instance.NewLoginProviderService(&FakeLoginProviderStore{}, FakeSealer{}, host, callbackDeTeste)

	view, err := svc.List(context.Background())

	require.NoError(t, err)
	assert.True(t, view.Locked)
	require.Len(t, view.Providers, 1)
	assert.Equal(t, "id-host", view.Providers[0].ClientID)
	assert.True(t, view.Providers[0].Locked)
	_, err = svc.Create(context.Background(), google())
	require.ErrorIs(t, err, instance.ErrLoginProvidersLocked)
	_, err = svc.Delete(context.Background(), "github")
	require.ErrorIs(t, err, instance.ErrLoginProvidersLocked)
}

func TestLoginProviders_Vigentes_SoOsLigadosComOSegredoAbertoEOCallback(t *testing.T) {
	svc, _ := servicoDeLogin(
		instance.LoginProviderRecord{ID: "google", Name: "Google", Issuer: "https://accounts.google.com", ClientID: "id", SealedSecret: "selado:s", Enabled: true},
		instance.LoginProviderRecord{ID: "github", Name: "GitHub", ClientID: "id-gh", SealedSecret: "selado:s2"}, //nolint:gosec // G101: segredo de teste
	)

	_, settings, err := svc.LoginProviders(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []loginprovider.Settings{{ID: "google", Name: "Google", Issuer: "https://accounts.google.com",
		ClientID: "id", ClientSecret: "s", RedirectURL: "https://painel.exemplo/auth/google/callback"}}, settings)
}

func TestLoginProviders_Vigentes_RevisaoMudaQuandoAListaMuda(t *testing.T) {
	svc, _ := servicoDeLogin()
	before, _, err := svc.LoginProviders(context.Background())
	require.NoError(t, err)

	_, err = svc.Create(context.Background(), google())
	require.NoError(t, err)
	after, _, err := svc.LoginProviders(context.Background())

	require.NoError(t, err)
	assert.NotEqual(t, before, after)
}

func TestLoginProviders_VigentesComSegredoIlegivel_PulaSoEsseProvedor(t *testing.T) {
	svc, _ := servicoDeLogin(
		instance.LoginProviderRecord{ID: "google", Issuer: "https://accounts.google.com", ClientID: "id", SealedSecret: "lixo", Enabled: true},
		instance.LoginProviderRecord{ID: "github", ClientID: "id-gh", SealedSecret: "selado:s2", Enabled: true}, //nolint:gosec // G101: segredo de teste
	)

	_, settings, err := svc.LoginProviders(context.Background())

	require.NoError(t, err)
	require.Len(t, settings, 1)
	assert.Equal(t, "github", settings[0].ID)
}

func TestLoginProviders_VigentesComBancoFora_DevolveOErro(t *testing.T) {
	store := &FakeLoginProviderStore{err: errors.New("banco fora")}
	svc := instance.NewLoginProviderService(store, FakeSealer{}, nil, callbackDeTeste)

	_, _, err := svc.LoginProviders(context.Background())

	require.Error(t, err)
}
