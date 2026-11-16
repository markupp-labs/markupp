package instance_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
)

// FakeEmbeddingStore guarda o registro do embedding em memória.
type FakeEmbeddingStore struct {
	record  instance.EmbeddingRecord
	saves   int
	reseals int
}

func (f *FakeEmbeddingStore) LoadEmbedding(context.Context) (instance.EmbeddingRecord, error) {
	return f.record, nil
}

func (f *FakeEmbeddingStore) ResealAPIKey(_ context.Context, old, fresh string) error {
	f.reseals++
	if f.record.Choice.SealedAPIKey == old {
		f.record.Choice.SealedAPIKey = fresh
	}
	return nil
}

func (f *FakeEmbeddingStore) SaveEmbeddingChoice(_ context.Context, choice instance.EmbeddingChoice) error {
	f.saves++
	choice.UpdatedAt = time.Unix(int64(f.saves), 0)
	f.record.Choice = choice
	return nil
}

// FakeSealer "cifra" pondo um prefixo, para o teste ver o que foi selado. O prefixo
// velho: é o da chave antiga do chaveiro, que abre e pede novo selo.
type FakeSealer struct{}

func (FakeSealer) Seal(plain string) (string, error) { return "selado:" + plain, nil }

func (FakeSealer) Open(sealed string) (string, bool, error) {
	if plain, ok := strings.CutPrefix(sealed, "selado:"); ok {
		return plain, false, nil
	}
	if plain, ok := strings.CutPrefix(sealed, "velho:"); ok {
		return plain, true, nil
	}
	return "", false, errors.New("nao selado")
}

func servico(record instance.EmbeddingRecord) (*instance.EmbeddingService, *FakeEmbeddingStore) {
	store := &FakeEmbeddingStore{record: record}
	return instance.NewEmbeddingService(store, FakeSealer{}), store
}

func remoto(chave *string) instance.EmbeddingUpdate {
	return instance.EmbeddingUpdate{Provider: "openai-compat", URL: "https://api.exemplo/v1", Model: "modelo-x", APIKey: chave}
}

func TestSettings_RegistroNovo_SemProvedorSemChaveSemEstado(t *testing.T) {
	svc, _ := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{Provider: "none"}})

	view, err := svc.Settings(context.Background())

	require.NoError(t, err)
	assert.Equal(t, instance.EmbeddingView{Provider: "none"}, view)
}

func TestSettings_NuncaDevolveAChave_SoQueEstaDefinida(t *testing.T) {
	svc, _ := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{Provider: "openai-compat", SealedAPIKey: "selado:sk"}})

	view, err := svc.Settings(context.Background())

	require.NoError(t, err)
	assert.True(t, view.APIKeySet)
}

func TestSettings_ComEstadoDoModeloETrava_DevolveOsDois(t *testing.T) {
	quando := time.Unix(100, 0)
	svc, _ := servico(instance.EmbeddingRecord{
		Choice: instance.EmbeddingChoice{Provider: "none"}, HostLocked: true, HostProvider: "local",
		Model: instance.ModelStatus{State: instance.ModelError, Error: "sem rede", ChangedAt: &quando},
	})

	view, err := svc.Settings(context.Background())

	require.NoError(t, err)
	assert.True(t, view.Locked)
	assert.Equal(t, "local", view.HostProvider)
	assert.Equal(t, &instance.ModelStatus{State: instance.ModelError, Error: "sem rede", ChangedAt: &quando}, view.ModelStatus)
}

func TestUpdate_RemotoComChave_SelaAChaveEGrava(t *testing.T) {
	svc, store := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{Provider: "none"}})
	chave := "sk-segredo"

	view, err := svc.Update(context.Background(), remoto(&chave))

	require.NoError(t, err)
	assert.Equal(t, "selado:sk-segredo", store.record.Choice.SealedAPIKey)
	assert.Equal(t, instance.EmbeddingView{Provider: "openai-compat", URL: "https://api.exemplo/v1", Model: "modelo-x", APIKeySet: true}, view)
}

func TestUpdate_SemChaveNoPedido_MantemAChaveAnterior(t *testing.T) {
	svc, store := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{Provider: "openai-compat", SealedAPIKey: "selado:velha"}}) // #nosec G101 -- valor fictício de teste

	_, err := svc.Update(context.Background(), remoto(nil))

	require.NoError(t, err)
	assert.Equal(t, "selado:velha", store.record.Choice.SealedAPIKey)
}

func TestUpdate_ChaveVazia_ApagaAChave(t *testing.T) {
	svc, store := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{Provider: "openai-compat", SealedAPIKey: "selado:velha"}}) // #nosec G101 -- valor fictício de teste
	vazia := ""

	_, err := svc.Update(context.Background(), remoto(&vazia))

	require.NoError(t, err)
	assert.Empty(t, store.record.Choice.SealedAPIKey)
}

func TestUpdate_TravadoPorQuemHospeda_RecusaSemGravar(t *testing.T) {
	svc, store := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{Provider: "none"}, HostLocked: true})

	_, err := svc.Update(context.Background(), instance.EmbeddingUpdate{Provider: "local"})

	assert.ErrorIs(t, err, instance.ErrHostLocked)
	assert.Zero(t, store.saves)
}

func TestUpdate_EscolhaInvalida_RecusaComOValorEOFormato(t *testing.T) {
	casos := map[string]struct {
		pedido   instance.EmbeddingUpdate
		esperado []string
	}{
		"provedor desconhecido": {instance.EmbeddingUpdate{Provider: "ollama"}, []string{`"ollama"`, "none, openai-compat, local"}},
		"remoto sem modelo":     {instance.EmbeddingUpdate{Provider: "openai-compat", URL: "https://api.exemplo/v1"}, []string{"model", "openai-compat"}},
		"remoto sem url":        {instance.EmbeddingUpdate{Provider: "openai-compat", Model: "m"}, []string{"url", "http"}},
		"url sem esquema http":  {instance.EmbeddingUpdate{Provider: "openai-compat", URL: "ftp://x/v1", Model: "m"}, []string{`"ftp://x/v1"`, "http"}},
		"modelo longo demais":   {instance.EmbeddingUpdate{Provider: "local", Model: strings.Repeat("m", 3000)}, []string{"3000", "2048"}},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			svc, store := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{Provider: "none"}})

			_, err := svc.Update(context.Background(), caso.pedido)

			require.ErrorIs(t, err, instance.ErrInvalidEmbedding)
			for _, trecho := range caso.esperado {
				assert.Contains(t, err.Error(), trecho)
			}
			assert.Zero(t, store.saves)
		})
	}
}

func TestPlainChoice_DecifraAChaveParaOIndexador(t *testing.T) {
	quando := time.Unix(7, 0)
	svc, _ := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{
		Provider: "openai-compat", URL: "u", Model: "m", SealedAPIKey: "selado:sk", UpdatedAt: quando,
	}})

	choice, err := svc.PlainChoice(context.Background())

	require.NoError(t, err)
	assert.Equal(t, instance.PlainEmbeddingChoice{Provider: "openai-compat", URL: "u", Model: "m", APIKey: "sk", UpdatedAt: quando}, choice)
}

func TestPlainChoice_ChaveDeOutraCifra_RetornaErro(t *testing.T) {
	svc, _ := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{Provider: "openai-compat", SealedAPIKey: "outra"}})

	_, err := svc.PlainChoice(context.Background())

	assert.Error(t, err)
}

func TestSettings_ChaveSeladaComAChaveAntiga_SelaDeNovoComAAtiva(t *testing.T) {
	svc, store := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{Provider: "openai-compat", SealedAPIKey: "velho:sk"}}) // #nosec G101 -- valor fictício de teste

	view, err := svc.Settings(context.Background())

	require.NoError(t, err)
	assert.True(t, view.APIKeySet)
	assert.Equal(t, "selado:sk", store.record.Choice.SealedAPIKey)
	assert.Zero(t, store.saves, "o novo selo não muda a escolha nem a revisão")
}

func TestSettings_ChaveJaComAAtivaOuIlegivel_NaoSelaDeNovo(t *testing.T) {
	for _, selada := range []string{"selado:sk", "de-outra-chave"} {
		svc, store := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{Provider: "openai-compat", SealedAPIKey: selada}})

		_, err := svc.Settings(context.Background())

		require.NoError(t, err, selada)
		assert.Zero(t, store.reseals, selada)
	}
}

func TestPlainChoice_ChaveSeladaComAChaveAntiga_DecifraSemSelarDeNovo(t *testing.T) {
	svc, store := servico(instance.EmbeddingRecord{Choice: instance.EmbeddingChoice{Provider: "openai-compat", SealedAPIKey: "velho:sk"}}) // #nosec G101 -- valor fictício de teste

	choice, err := svc.PlainChoice(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "sk", choice.APIKey)
	assert.Zero(t, store.reseals, "o indexador de embedding só decifra")
}
