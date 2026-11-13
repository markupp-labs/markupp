package embedding_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
)

// fakeEmbeddingsServer responde como o /embeddings da API da OpenAI, com os itens
// fora de ordem, e guarda o pedido recebido.
type fakeEmbeddingsServer struct {
	status        int
	authorization string
	body          struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	path string
}

func (f *fakeEmbeddingsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.path, f.authorization = r.URL.Path, r.Header.Get("Authorization")
	_ = json.NewDecoder(r.Body).Decode(&f.body)
	if f.status != 0 {
		http.Error(w, "cota esgotada", f.status)
		return
	}
	_, _ = w.Write([]byte(`{"data":[{"index":1,"embedding":[0,2]},{"index":0,"embedding":[3,4]}]}`))
}

func provedorRemoto(t *testing.T, server *fakeEmbeddingsServer) embedding.Provider {
	t.Helper()
	ts := httptest.NewServer(server)
	t.Cleanup(ts.Close)
	return embedding.NewOpenAICompatProvider(ts.Client(), ts.URL+"/v1/", "modelo-x", "chave")
}

func TestOpenAICompat_Embed_OrdenaPeloIndiceENormaliza(t *testing.T) {
	server := &fakeEmbeddingsServer{}

	vetores, err := provedorRemoto(t, server).Embed(context.Background(), []string{"um", "dois"})

	require.NoError(t, err)
	assert.InDeltaSlice(t, []float32{0.6, 0.8}, vetores[0], 1e-6)
	assert.InDeltaSlice(t, []float32{0, 1}, vetores[1], 1e-6)
	assert.Equal(t, "/v1/embeddings", server.path)
	assert.Equal(t, "Bearer chave", server.authorization)
	assert.Equal(t, "modelo-x", server.body.Model)
	assert.Equal(t, []string{"um", "dois"}, server.body.Input)
}

func TestOpenAICompat_Embed_StatusDeErro_RetornaErroComOStatus(t *testing.T) {
	_, err := provedorRemoto(t, &fakeEmbeddingsServer{status: http.StatusTooManyRequests}).Embed(context.Background(), []string{"um"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
}

func TestOpenAICompat_Embed_MenosItensQueTextos_RetornaErro(t *testing.T) {
	_, err := provedorRemoto(t, &fakeEmbeddingsServer{}).Embed(context.Background(), []string{"um", "dois", "tres"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "3")
}

func TestOpenAICompat_Name_IdentificaModeloEEndereco(t *testing.T) {
	p := embedding.NewOpenAICompatProvider(http.DefaultClient, "https://api.exemplo/v1", "modelo-x", "")

	assert.Equal(t, "openai-compat:modelo-x@https://api.exemplo/v1", p.Name())
}

func TestOpenAICompat_TokenSpans_QuatroLetrasPorToken(t *testing.T) {
	p := embedding.NewOpenAICompatProvider(http.DefaultClient, "https://api.exemplo/v1", "modelo-x", "")

	assert.Equal(t, embedding.CharSpans("abcdefgh"), p.TokenSpans("abcdefgh"))
}
