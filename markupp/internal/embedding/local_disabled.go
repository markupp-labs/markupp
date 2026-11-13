//go:build !ORT

package embedding

import (
	"context"
	"errors"
)

// NewLocalProvider falha: este binário foi compilado sem o ONNX Runtime.
func NewLocalProvider(context.Context, string, ModelFile) (Provider, func() error, error) {
	return nil, nil, errors.New("local embedding provider needs the ONNX Runtime build: run embed-loop from the markupp-embedder image")
}
