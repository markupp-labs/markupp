package embedding

import (
	"fmt"
	"os"

	"github.com/gomlx/go-huggingface/tokenizers/api"
	"github.com/gomlx/go-huggingface/tokenizers/hftokenizer"
)

// Tokenizer marca os tokens de um texto com o tokenizer.json do modelo local, para os
// pedaços caberem na janela dele. É uma instância própria: a do hugot não devolve as
// posições.
type Tokenizer struct{ tk *hftokenizer.Tokenizer }

// LoadTokenizer lê o tokenizer.json de path, sem tokens especiais e com as posições.
func LoadTokenizer(path string) (*Tokenizer, error) {
	content, err := os.ReadFile(path) //nolint:gosec // o caminho vem da pasta do modelo na config
	if err != nil {
		return nil, fmt.Errorf("read tokenizer %s: %w", path, err)
	}
	tk, err := hftokenizer.NewFromContent(nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse tokenizer %s, expected a Hugging Face tokenizer.json: %w", path, err)
	}
	if err := tk.With(api.EncodeOptions{IncludeSpans: true}); err != nil {
		return nil, fmt.Errorf("configure tokenizer %s: %w", path, err)
	}
	return &Tokenizer{tk: tk}, nil
}

// Spans devolve o início e o fim, em bytes, de cada token de text.
func (t *Tokenizer) Spans(text string) [][2]int {
	out := t.tk.EncodeWithAnnotations(text)
	spans := make([][2]int, 0, len(out.Spans))
	for _, s := range out.Spans {
		spans = append(spans, [2]int{s.Start, s.End})
	}
	return spans
}
