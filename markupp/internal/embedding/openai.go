package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// openAICompatProvider pede os vetores a um endpoint /embeddings no formato da API da
// OpenAI, que outros servidores também falam.
type openAICompatProvider struct {
	client  *http.Client
	baseURL string
	model   string
	apiKey  string
}

// NewOpenAICompatProvider monta o provedor remoto de baseURL, sem /embeddings no fim,
// com o modelo model e a chave apiKey, vazia quando o servidor não pede.
func NewOpenAICompatProvider(client *http.Client, baseURL, model, apiKey string) Provider {
	return &openAICompatProvider{client: client, baseURL: strings.TrimSuffix(baseURL, "/"), model: model, apiKey: apiKey}
}

func (p *openAICompatProvider) Name() string {
	return "openai-compat:" + p.model + "@" + p.baseURL
}

// TokenSpans conta quatro letras por token: o tokenizador do servidor não é exposto.
func (p *openAICompatProvider) TokenSpans(text string) [][2]int { return CharSpans(text) }

type embeddingsResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (p *openAICompatProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := p.post(ctx, texts)
	if err != nil {
		return nil, err
	}
	vectors := make([][]float32, len(texts))
	for _, item := range resp.Data {
		if item.Index >= 0 && item.Index < len(texts) {
			vectors[item.Index] = Normalize(item.Embedding)
		}
	}
	for i, v := range vectors {
		if len(v) == 0 {
			return nil, fmt.Errorf("embeddings from %s miss text %d of %d: expected one embedding per input", p.baseURL, i, len(texts))
		}
	}
	return vectors, nil
}

func (p *openAICompatProvider) post(ctx context.Context, texts []string) (embeddingsResponse, error) {
	var out embeddingsResponse
	body, err := json.Marshal(map[string]any{"model": p.model, "input": texts})
	if err != nil {
		return out, fmt.Errorf("encode embeddings request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return out, fmt.Errorf("build embeddings request to %s: %w", p.baseURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	return out, p.do(req, &out)
}

func (p *openAICompatProvider) do(req *http.Request, out *embeddingsResponse) error {
	res, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("post embeddings to %s: %w", p.baseURL, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("embeddings from %s answered %d, expected 200: %s", p.baseURL, res.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("decode embeddings from %s, expected {\"data\":[{\"index\",\"embedding\"}]}: %w", p.baseURL, err)
	}
	return nil
}
