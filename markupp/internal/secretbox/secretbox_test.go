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

func chaveiro(t *testing.T, ativa string, antigas ...string) *secretbox.Keyring {
	t.Helper()
	ring, err := secretbox.NewKeyring(ativa, antigas...)
	require.NoError(t, err)
	return ring
}

func TestKeyring_SeladoComAAtiva_AbreSemPedirNovoSelo(t *testing.T) {
	ring := chaveiro(t, "chave-nova", "chave-do-banco")
	selado, err := ring.Seal("sk")
	require.NoError(t, err)

	aberto, stale, err := ring.Open(selado)

	require.NoError(t, err)
	assert.Equal(t, "sk", aberto)
	assert.False(t, stale)
}

func TestKeyring_SeladoComAAntiga_AbreEPedeNovoSelo(t *testing.T) {
	selado, err := caixa(t, "chave-do-banco").Seal("sk")
	require.NoError(t, err)

	aberto, stale, err := chaveiro(t, "chave-nova", "chave-do-banco").Open(selado)

	require.NoError(t, err)
	assert.Equal(t, "sk", aberto)
	assert.True(t, stale)
}

func TestKeyring_SeladoComOutraChave_Recusa(t *testing.T) {
	selado, err := caixa(t, "outra").Seal("sk")
	require.NoError(t, err)

	_, _, err = chaveiro(t, "chave-nova", "chave-do-banco").Open(selado)

	assert.Error(t, err)
}

func TestNewKeyring_AntigaIgualAAtiva_NaoPedeNovoSelo(t *testing.T) {
	selado, err := caixa(t, "mesma").Seal("sk")
	require.NoError(t, err)

	_, stale, err := chaveiro(t, "mesma", "mesma").Open(selado)

	require.NoError(t, err)
	assert.False(t, stale)
}
