package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// ModelFile é um arquivo do modelo local, pelo caminho no repositório e o SHA-256
// esperado.
type ModelFile struct {
	Path   string
	SHA256 string
}

// ModelFetcher baixa um arquivo pela URL.
type ModelFetcher interface {
	Fetch(ctx context.Context, url string) (io.ReadCloser, error)
}

// O modelo local (ADR-0056), fixado numa revisão do Hugging Face. Os hashes são os do
// LFS, e o de config.json foi tirado do arquivo da mesma revisão.
const (
	GraniteModel    = "ibm-granite/granite-embedding-311m-multilingual-r2"
	GraniteRevision = "44399559930365213510b1ee2eb15ded83374f0e"
	GraniteBaseURL  = "https://huggingface.co/" + GraniteModel + "/resolve/" + GraniteRevision
)

var (
	graniteTokenizer = ModelFile{Path: "tokenizer.json", SHA256: "0087c868b33bad550a78a08d19798cfd7f713cde4f020803b8f51f405503e15f"}
	graniteConfig    = ModelFile{Path: "config.json", SHA256: "e1e3fc842a8e0537e25d6e4c93879698b92ae96722e8c162bef334b57978a3b0"}
	graniteQuantized = ModelFile{Path: "onnx/model_quint8_avx2.onnx", SHA256: "f1fdd44e7e1ac51f12ab7957c7bd092e064d596c288513bf9d326842f669edee"}
	graniteFull      = ModelFile{Path: "onnx/model.onnx", SHA256: "75f9f258bf5013f5fe8a4dad61dd0fd16ac0cbaa7a106e3d3f41c2d04a42d541"}
)

// GraniteFiles lista os arquivos do modelo local, com os pesos por último: os
// quantizados em x86 com AVX2, os cheios nas outras CPUs.
func GraniteFiles(avx2 bool) []ModelFile {
	weights := graniteFull
	if avx2 {
		weights = graniteQuantized
	}
	return []ModelFile{graniteTokenizer, graniteConfig, weights}
}

// EnsureModelFiles deixa em dir cada arquivo de files com o hash esperado, baixando de
// baseURL o que falta ou difere. Recusa o download de outro hash, sem deixar arquivo.
func EnsureModelFiles(ctx context.Context, fetcher ModelFetcher, dir, baseURL string, files []ModelFile) error {
	for _, file := range files {
		target := filepath.Join(dir, filepath.FromSlash(file.Path))
		if fileHash(target) == file.SHA256 {
			continue
		}
		if err := download(ctx, fetcher, baseURL+"/"+file.Path, target, file); err != nil {
			return err
		}
	}
	return nil
}

// fileHash é o SHA-256 de path, vazio quando não dá para ler.
func fileHash(path string) string {
	f, err := os.Open(path) //nolint:gosec // o caminho vem da lista fixa de arquivos do modelo
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// download grava url num temporário ao lado de target e só o renomeia com o hash certo.
func download(ctx context.Context, fetcher ModelFetcher, url, target string, file ModelFile) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("create model directory for %s: %w", file.Path, err)
	}
	body, err := fetcher.Fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("download model file %s from %s: %w", file.Path, url, err)
	}
	defer func() { _ = body.Close() }()
	tmp, err := os.CreateTemp(filepath.Dir(target), filepath.Base(target)+".*.part")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", file.Path, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	got, err := copyHashing(tmp, body)
	if err != nil {
		return fmt.Errorf("write model file %s: %w", file.Path, err)
	}
	if got != file.SHA256 {
		return fmt.Errorf("model file %s from %s has sha256 %s, expected %s: refusing it", file.Path, url, got, file.SHA256)
	}
	return os.Rename(tmp.Name(), target)
}

// copyHashing copia src para dst, fecha dst e devolve o SHA-256 do que passou.
func copyHashing(dst *os.File, src io.Reader) (string, error) {
	h := sha256.New()
	_, err := io.Copy(io.MultiWriter(dst, h), src)
	if closeErr := dst.Close(); err == nil {
		err = closeErr
	}
	return hex.EncodeToString(h.Sum(nil)), err
}

// httpModelFetcher baixa os arquivos do modelo por HTTP.
type httpModelFetcher struct{ client *http.Client }

// NewHTTPModelFetcher baixa com client, que segue o redirecionamento do Hugging Face
// para o armazenamento dos arquivos grandes.
func NewHTTPModelFetcher(client *http.Client) ModelFetcher { return httpModelFetcher{client: client} }

func (f httpModelFetcher) Fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, errors.New("answered " + res.Status + ", expected 200 OK")
	}
	return res.Body, nil
}
