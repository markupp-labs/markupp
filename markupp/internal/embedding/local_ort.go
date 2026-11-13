//go:build ORT

package embedding

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

// localProvider roda o granite-embedding no ONNX Runtime pelo hugot, com o pooling CLS.
type localProvider struct {
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
	name     string
}

// NewLocalProvider sobe o modelo baixado em dir com os pesos de weights, e devolve o
// provedor e a função que libera o ONNX Runtime. A libonnxruntime.so fica em /usr/lib.
func NewLocalProvider(ctx context.Context, dir string, weights ModelFile) (Provider, func() error, error) {
	session, err := hugot.NewORTSession(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("start ONNX Runtime, expected /usr/lib/libonnxruntime.so: %w", err)
	}
	cfg := hugot.FeatureExtractionConfig{ModelPath: dir, Name: "markupp-embedding", OnnxFilename: path.Base(weights.Path)}
	pipeline, err := session.NewPipeline(cfg)
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("load embedding model %s from %s: %w", weights.Path, dir, err), session.Destroy())
	}
	name := "local:" + GraniteModel + "@" + GraniteRevision + "/" + weights.Path
	return &localProvider{session: session, pipeline: pipeline, name: name}, session.Destroy, nil
}

func (p *localProvider) Name() string { return p.name }

// TokenSpans tokeniza com o tokenizador do modelo e deixa de fora os tokens especiais.
func (p *localProvider) TokenSpans(text string) [][2]int {
	out := p.pipeline.Model.Tokenizer.GoTokenizer.Tokenizer.EncodeWithAnnotations(text)
	spans := make([][2]int, 0, len(out.Spans))
	for i, span := range out.Spans {
		if i < len(out.SpecialTokensMask) && out.SpecialTokensMask[i] != 0 {
			continue
		}
		spans = append(spans, [2]int{span.Start, span.End})
	}
	return spans
}

// Embed roda o modelo sem pooling e fica com o CLS de cada texto, normalizado.
func (p *localProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	raw, err := p.pipeline.RunRaw(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("run embedding model on %d chunks: %w", len(texts), err)
	}
	if raw.HiddenStates == nil {
		return nil, fmt.Errorf("embedding model output %q has shape %v, expected [batch, tokens, dimensions]", raw.OutputName, raw.Dimensions)
	}
	return CLSVectors(raw.HiddenStates)
}
