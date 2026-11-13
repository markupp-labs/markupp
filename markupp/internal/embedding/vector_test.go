package embedding_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
)

func TestNormalize_DeixaONormaUm(t *testing.T) {
	assert.InDeltaSlice(t, []float32{0.6, 0.8}, embedding.Normalize([]float32{3, 4}), 1e-6)
}

func TestNormalize_VetorZero_FicaZero(t *testing.T) {
	assert.Equal(t, []float32{0, 0}, embedding.Normalize([]float32{0, 0}))
}

func TestMeanNormalized_MediaDosPedacosRenormalizada(t *testing.T) {
	media, err := embedding.MeanNormalized([][]float32{{1, 0}, {0, 1}})

	require.NoError(t, err)
	assert.InDeltaSlice(t, []float32{0.70710677, 0.70710677}, media, 1e-6)
}

func TestMeanNormalized_DimensoesDiferentes_RetornaErroComAsDimensoes(t *testing.T) {
	_, err := embedding.MeanNormalized([][]float32{{1, 0}, {0, 1, 0}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "3")
	assert.Contains(t, err.Error(), "2")
}

func TestCLSVectors_PegaOPrimeiroTokenDeCadaTextoNormalizado(t *testing.T) {
	ocultos := [][][]float32{{{3, 4}, {9, 9}}, {{0, 2}, {1, 1}}}

	vetores, err := embedding.CLSVectors(ocultos)

	require.NoError(t, err)
	assert.InDeltaSlice(t, []float32{0.6, 0.8}, vetores[0], 1e-6)
	assert.InDeltaSlice(t, []float32{0, 1}, vetores[1], 1e-6)
}

func TestCLSVectors_TextoSemToken_RetornaErro(t *testing.T) {
	_, err := embedding.CLSVectors([][][]float32{{}})

	require.Error(t, err)
}
