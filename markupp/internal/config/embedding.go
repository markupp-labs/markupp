package config

import (
	"fmt"
	"strings"
)

// Provedores do indexador de embedding (ADR-0053). Sem provedor, que é o padrão, não
// há ligação por sentido e nada é baixado.
const (
	EmbeddingNone         = "none"
	EmbeddingOpenAICompat = "openai-compat"
	EmbeddingLocal        = "local"
)

// Embedding são os parâmetros do indexador de embedding, lidos só por ele.
type Embedding struct {
	// Provider é none, openai-compat (um endpoint /embeddings remoto) ou local (o modelo
	// fixo no ONNX Runtime, baixado na primeira subida).
	Provider string
	// URL é a base do endpoint remoto, sem /embeddings, e Model o modelo pedido a ele.
	URL   string
	Model string
	// APIKey vai no Authorization do endpoint remoto, quando ele pede.
	APIKey string
	// ModelDir guarda os arquivos do modelo local.
	ModelDir string
	// ChunkTokens é o tamanho do pedaço de nota que vira um vetor (ADR-0056).
	ChunkTokens int
}

// LoadEmbedding lê as variáveis MARKUPP_EMBEDDING_* e confere que o provedor escolhido
// tem o que precisa.
func LoadEmbedding(lookup LookupFunc) (Embedding, error) {
	cfg := Embedding{Provider: EmbeddingNone, ModelDir: "/var/lib/markupp/models", ChunkTokens: 512}
	for key, target := range map[string]*string{
		"MARKUPP_EMBEDDING_PROVIDER": &cfg.Provider, "MARKUPP_EMBEDDING_URL": &cfg.URL, "MARKUPP_EMBEDDING_MODEL": &cfg.Model,
		"MARKUPP_EMBEDDING_API_KEY": &cfg.APIKey, "MARKUPP_EMBEDDING_MODEL_DIR": &cfg.ModelDir,
	} {
		readString(lookup, key, target)
	}
	chunk := int64(cfg.ChunkTokens)
	if err := readBounded(lookup, "MARKUPP_EMBEDDING_CHUNK_TOKENS", 16, 8192, "between 16 and 8192", &chunk); err != nil {
		return Embedding{}, err
	}
	cfg.ChunkTokens = int(chunk)
	return cfg, checkEmbeddingProvider(cfg)
}

func checkEmbeddingProvider(cfg Embedding) error {
	switch cfg.Provider {
	case EmbeddingNone, EmbeddingLocal:
		return nil
	case EmbeddingOpenAICompat:
		if cfg.URL == "" || cfg.Model == "" {
			return fmt.Errorf("MARKUPP_EMBEDDING_URL=%q and MARKUPP_EMBEDDING_MODEL=%q, expected both with provider %s", cfg.URL, cfg.Model, EmbeddingOpenAICompat)
		}
		return nil
	}
	return fmt.Errorf("MARKUPP_EMBEDDING_PROVIDER=%q, expected one of %s", cfg.Provider,
		strings.Join([]string{EmbeddingNone, EmbeddingOpenAICompat, EmbeddingLocal}, ", "))
}
