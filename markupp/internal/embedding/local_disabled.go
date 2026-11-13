//go:build !ORT

package embedding

import (
	"context"
	"errors"
)

// OpenLocalProvider falha antes de baixar qualquer arquivo: este binário foi
// compilado sem o ONNX Runtime.
func OpenLocalProvider(context.Context, ModelFetcher, string) (Provider, func() error, error) {
	return nil, nil, errors.New("local embedding provider needs the ONNX Runtime build: run embed-loop from the markupp-embedder image")
}
