package embedding_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
)

// tokenizadorWordPiece é um tokenizer.json mínimo, no formato do Hugging Face, com o
// pós-processamento que põe [CLS] e [SEP] como o do modelo local.
//
//nolint:gosec // vocabulário de teste, não é credencial
const tokenizadorWordPiece = `{
  "version": "1.0",
  "added_tokens": [
    {"id": 101, "content": "[CLS]", "special": true},
    {"id": 102, "content": "[SEP]", "special": true}
  ],
  "normalizer": {"type": "BertNormalizer", "lowercase": true},
  "pre_tokenizer": {"type": "BertPreTokenizer"},
  "post_processor": {"type": "BertProcessing", "sep": ["[SEP]", 102], "cls": ["[CLS]", 101]},
  "decoder": {"type": "WordPiece", "prefix": "##"},
  "model": {
    "type": "WordPiece", "unk_token": "[UNK]", "continuing_subword_prefix": "##", "max_input_chars_per_word": 100,
    "vocab": {"[UNK]": 100, "[CLS]": 101, "[SEP]": 102, "hello": 1, "world": 2, "test": 3, "##ing": 4}
  }
}`

func TestLoadTokenizer_MarcaCadaTokenSemOsEspeciais(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokenizer.json")
	require.NoError(t, os.WriteFile(path, []byte(tokenizadorWordPiece), 0o600))

	tk, err := embedding.LoadTokenizer(path)

	require.NoError(t, err)
	assert.Equal(t, [][2]int{{0, 5}, {6, 11}, {12, 16}, {16, 19}}, tk.Spans("hello world testing"),
		"sem as posicoes o indexador nao corta pedaco nenhum e a nota fica sem vetor")
}

func TestLoadTokenizer_ArquivoAusente_RetornaErroComOCaminho(t *testing.T) {
	_, err := embedding.LoadTokenizer("/nao/existe/tokenizer.json")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "/nao/existe/tokenizer.json")
}
