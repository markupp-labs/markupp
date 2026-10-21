package api

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// errInvalidQuery marca o parâmetro de consulta inválido, que vira 400.
var errInvalidQuery = errors.New("parametro de consulta invalido")

// queryReader lê parâmetros opcionais da query string e guarda o primeiro inválido
// em err; depois dele, as leituras devolvem o valor zero.
type queryReader struct {
	values url.Values
	err    error
}

// boolean lê name como true ou false, nil quando ausente.
func (qr *queryReader) boolean(name string) *bool {
	raw := qr.values.Get(name)
	if raw == "" || qr.err != nil {
		return nil
	}
	if raw != "true" && raw != "false" {
		qr.err = fmt.Errorf("%w: %s %q, esperado true ou false", errInvalidQuery, name, raw)
		return nil
	}
	v := raw == "true"
	return &v
}

// int64 lê name como inteiro, nil quando ausente.
func (qr *queryReader) int64(name string) *int64 {
	raw := qr.values.Get(name)
	if raw == "" || qr.err != nil {
		return nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		qr.err = fmt.Errorf("%w: %s %q, esperado inteiro", errInvalidQuery, name, raw)
		return nil
	}
	return &v
}

// intBetween lê name como inteiro de lo a hi, fallback quando ausente.
func (qr *queryReader) intBetween(name string, fallback, lo, hi int) int {
	raw := qr.values.Get(name)
	if raw == "" || qr.err != nil {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < lo || v > hi {
		qr.err = fmt.Errorf("%w: %s %q, esperado inteiro de %d a %d", errInvalidQuery, name, raw, lo, hi)
		return 0
	}
	return v
}

// oneOf lê name, que precisa ser um dos valores aceitos, vazio quando ausente.
func (qr *queryReader) oneOf(name string, accepted ...string) string {
	raw := qr.values.Get(name)
	if raw == "" || qr.err != nil {
		return ""
	}
	if !slices.Contains(accepted, raw) {
		qr.err = fmt.Errorf("%w: %s %q, esperado um de %s", errInvalidQuery, name, raw, strings.Join(accepted, ", "))
		return ""
	}
	return raw
}

// page lê offset e limit como pageParams.
func (qr *queryReader) page() (offset, limit int) {
	if qr.err != nil {
		return 0, 0
	}
	offset, limit, qr.err = pageParams(qr.values)
	return offset, limit
}
