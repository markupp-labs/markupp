package embedding

import (
	"errors"
	"fmt"
	"math"
)

// Normalize devolve v com norma L2 um; o vetor zero continua zero.
func Normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	out := make([]float32, len(v))
	if sum == 0 {
		return out
	}
	norm := math.Sqrt(sum)
	for i, x := range v {
		out[i] = float32(float64(x) / norm)
	}
	return out
}

// MeanNormalized é a média dos vetores dos pedaços, normalizada de novo (ADR-0056).
func MeanNormalized(vectors [][]float32) ([]float32, error) {
	if len(vectors) == 0 {
		return nil, errors.New("mean of zero vectors: expected at least one chunk vector")
	}
	mean := make([]float32, len(vectors[0]))
	for _, v := range vectors {
		if len(v) != len(mean) {
			return nil, fmt.Errorf("chunk vector has %d dimensions, expected %d like the first chunk", len(v), len(mean))
		}
		for i, x := range v {
			mean[i] += x
		}
	}
	return Normalize(mean), nil
}

// CLSVectors pega o estado oculto do primeiro token, o CLS, de cada texto, e o
// normaliza: é o pooling do granite-embedding.
func CLSVectors(hidden [][][]float32) ([][]float32, error) {
	vectors := make([][]float32, 0, len(hidden))
	for i, tokens := range hidden {
		if len(tokens) == 0 {
			return nil, fmt.Errorf("text %d has no hidden states, expected [tokens][dimensions]", i)
		}
		vectors = append(vectors, Normalize(tokens[0]))
	}
	return vectors, nil
}
