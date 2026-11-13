package secretbox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/secretbox"
)

func caixa(t *testing.T, chave string) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New(chave)
	require.NoError(t, err)
	return box
}

func TestSealOpen_IdaEVolta_DevolveOTextoOriginal(t *testing.T) {
	box := caixa(t, "chave-da-instancia")

	selado, err := box.Seal("sk-segredo")
	require.NoError(t, err)
	aberto, err := box.Open(selado)

	require.NoError(t, err)
	assert.Equal(t, "sk-segredo", aberto)
	assert.NotContains(t, selado, "sk-segredo")
}

func TestSeal_MesmoTextoDuasVezes_SelaDiferenteComNonceAleatorio(t *testing.T) {
	box := caixa(t, "chave-da-instancia")

	primeiro, err := box.Seal("sk-segredo")
	require.NoError(t, err)
	segundo, err := box.Seal("sk-segredo")
	require.NoError(t, err)

	assert.NotEqual(t, primeiro, segundo)
}

func TestOpen_OutraChave_Recusa(t *testing.T) {
	selado, err := caixa(t, "chave-a").Seal("sk-segredo")
	require.NoError(t, err)

	_, err = caixa(t, "chave-b").Open(selado)

	assert.Error(t, err)
}

func TestOpen_ForaDoFormato_RetornaErroComOFormatoEsperado(t *testing.T) {
	_, err := caixa(t, "chave").Open("não é base64!")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "base64")
}

func TestOpen_CurtoDemais_RetornaErroComOTamanho(t *testing.T) {
	_, err := caixa(t, "chave").Open("YWJj")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "3 bytes")
}

func TestNew_ChaveVazia_Recusa(t *testing.T) {
	_, err := secretbox.New("")

	assert.Error(t, err)
}
