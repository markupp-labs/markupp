package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// valorLongoDoCliente tem caracteres fora da RFC 6749 e passa do limite de oauth.ClientValue.
var valorLongoDoCliente = `ação"\` + strings.Repeat("x", 200)

// requireDescricaoRFC6749 lê o error_description e exige que ele fique no conjunto da
// RFC 6749 (%x20-21 / %x23-5B / %x5D-7E) e com o valor do cliente limitado.
func requireDescricaoRFC6749(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Description string `json:"error_description"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	for _, r := range body.Description {
		require.True(t, r >= 0x20 && r <= 0x7e && r != '"' && r != '\\', "caractere %q fora da RFC 6749 em %q", r, body.Description)
	}
	assert.NotContains(t, body.Description, strings.Repeat("x", 100))
	return body.Description
}

func TestAuthorize_ParametroRepetidoComNomeDoCliente_DescricaoNaRFC6749(t *testing.T) {
	rec := authorizeRequest(&fakeOAuthService{}, valorLongoDoCliente+"=a&"+valorLongoDoCliente+"=b")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	requireDescricaoRFC6749(t, rec)
}

func TestAuthorize_QueryMalformada_DescricaoNaRFC6749(t *testing.T) {
	rec := authorizeRequest(&fakeOAuthService{}, "client_id=c1&state=%zz")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, requireDescricaoRFC6749(t, rec), "%zz")
}

func TestToken_ContentTypeDoCliente_DescricaoNaRFC6749(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader("grant_type=authorization_code"))
	req.Header.Set("Content-Type", "text/plain; x="+valorLongoDoCliente)

	rec := serveOAuth(&fakeOAuthService{}, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, requireDescricaoRFC6749(t, rec), "'text/plain")
}

func TestToken_GrantTypeDoCliente_DescricaoNaRFC6749(t *testing.T) {
	form := codeForm()
	form.Set("grant_type", valorLongoDoCliente)

	rec := tokenRequest(&fakeOAuthService{}, form)

	assert.Contains(t, rec.Body.String(), `"error":"unsupported_grant_type"`)
	assert.Contains(t, requireDescricaoRFC6749(t, rec), "'a??o??x")
}

func TestToken_ParametroRepetidoComNomeDoCliente_DescricaoNaRFC6749(t *testing.T) {
	form := codeForm()
	form.Add(valorLongoDoCliente, "a")
	form.Add(valorLongoDoCliente, "b")

	rec := tokenRequest(&fakeOAuthService{}, form)

	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
	requireDescricaoRFC6749(t, rec)
}
