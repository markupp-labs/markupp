package embedding_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
)

// fakeModelFetcher serve o conteúdo de cada URL e conta os downloads.
type fakeModelFetcher struct {
	files     map[string][]byte
	downloads []string
}

func (f *fakeModelFetcher) Fetch(_ context.Context, url string) (io.ReadCloser, error) {
	f.downloads = append(f.downloads, url)
	return io.NopCloser(bytes.NewReader(f.files[url])), nil
}

func hashDe(conteudo string) string {
	sum := sha256.Sum256([]byte(conteudo))
	return hex.EncodeToString(sum[:])
}

const urlDoArquivo = "https://hub.exemplo/repo/resolve/rev/onnx/model.onnx"

func TestEnsureModelFiles_BaixaEGuardaQuandoOHashConfere(t *testing.T) {
	dir := t.TempDir()
	fetcher := &fakeModelFetcher{files: map[string][]byte{urlDoArquivo: []byte("pesos")}}

	err := embedding.EnsureModelFiles(context.Background(), fetcher, dir, "https://hub.exemplo/repo/resolve/rev",
		[]embedding.ModelFile{{Path: "onnx/model.onnx", SHA256: hashDe("pesos")}})

	require.NoError(t, err)
	gravado, err := os.ReadFile(filepath.Join(dir, "onnx", "model.onnx"))
	require.NoError(t, err)
	assert.Equal(t, "pesos", string(gravado))
}

func TestEnsureModelFiles_HashDiferente_RecusaENaoDeixaArquivo(t *testing.T) {
	dir := t.TempDir()
	fetcher := &fakeModelFetcher{files: map[string][]byte{urlDoArquivo: []byte("adulterado")}}

	err := embedding.EnsureModelFiles(context.Background(), fetcher, dir, "https://hub.exemplo/repo/resolve/rev",
		[]embedding.ModelFile{{Path: "onnx/model.onnx", SHA256: hashDe("pesos")}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), hashDe("pesos"))
	assert.Contains(t, err.Error(), "onnx/model.onnx")
	entradas, _ := os.ReadDir(filepath.Join(dir, "onnx"))
	assert.Empty(t, entradas, "nem o arquivo nem o temporario ficam")
}

func TestEnsureModelFiles_ArquivoJaGuardadoComOHash_NaoBaixaDeNovo(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "onnx"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "onnx", "model.onnx"), []byte("pesos"), 0o644))
	fetcher := &fakeModelFetcher{}

	err := embedding.EnsureModelFiles(context.Background(), fetcher, dir, "https://hub.exemplo/repo/resolve/rev",
		[]embedding.ModelFile{{Path: "onnx/model.onnx", SHA256: hashDe("pesos")}})

	require.NoError(t, err)
	assert.Empty(t, fetcher.downloads)
}

func TestGraniteFiles_ComAVX2_UsaOModeloQuantizado(t *testing.T) {
	files := embedding.GraniteFiles(true)

	assert.Equal(t, "onnx/model_quint8_avx2.onnx", files[len(files)-1].Path)
	assert.Equal(t, "f1fdd44e7e1ac51f12ab7957c7bd092e064d596c288513bf9d326842f669edee", files[len(files)-1].SHA256)
}

func TestGraniteFiles_SemAVX2_UsaOModeloCheio(t *testing.T) {
	files := embedding.GraniteFiles(false)

	assert.Equal(t, "onnx/model.onnx", files[len(files)-1].Path)
	assert.Equal(t, "75f9f258bf5013f5fe8a4dad61dd0fd16ac0cbaa7a106e3d3f41c2d04a42d541", files[len(files)-1].SHA256)
}
