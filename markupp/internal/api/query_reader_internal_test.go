package api

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryReader_AusentesFicamNoValorZero(t *testing.T) {
	qr := queryReader{values: url.Values{}}

	assert.Nil(t, qr.boolean("orphan"))
	assert.Nil(t, qr.int64("community"))
	assert.Empty(t, qr.oneOf("sort", "pagerank"))
	offset, limit := qr.page()
	assert.Equal(t, []int{0, 10}, []int{offset, limit})
	assert.NoError(t, qr.err)
}

func TestQueryReader_GuardaOPrimeiroInvalidoComOValorEOFormato(t *testing.T) {
	qr := queryReader{values: url.Values{"orphan": {"1"}, "sort": {"titulo"}}}

	assert.Nil(t, qr.boolean("orphan"))
	assert.Empty(t, qr.oneOf("sort", "pagerank", "degree"))

	require.ErrorIs(t, qr.err, errInvalidQuery)
	assert.Contains(t, qr.err.Error(), `orphan "1", esperado true ou false`, "so o primeiro invalido")
}

func TestQueryReader_LeOsValoresAceitos(t *testing.T) {
	qr := queryReader{values: url.Values{"orphan": {"false"}, "community": {"-3"}, "sort": {"degree"}}}

	assert.Equal(t, false, *qr.boolean("orphan"))
	assert.Equal(t, int64(-3), *qr.int64("community"))
	assert.Equal(t, "degree", qr.oneOf("sort", "pagerank", "degree"))
	assert.NoError(t, qr.err)
}
