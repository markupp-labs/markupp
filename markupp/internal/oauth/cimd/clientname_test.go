package cimd_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth/cimd"
)

func TestSanitizeClientName_NomeAceito_DevolveSaneado(t *testing.T) {
	casos := map[string]struct{ bruto, esperado string }{
		"simples":                         {"Agente", "Agente"},
		"espacos juntados":                {"  Agente \n\t Bom  ", "Agente Bom"},
		"controle removido":               {"Agente\x00Bom\x7f", "AgenteBom"},
		"formatacao removida":             {"Agente\u202e\u200bBom", "AgenteBom"},
		"acento combinante":               {"Cafe\u0301", "Cafe\u0301"},
		"duas marcas seguidas":            {"a\u0301\u0302", "a\u0301\u0302"},
		"100 caracteres depois de sanear": {strings.Repeat("a", 100) + strings.Repeat("\u200b", 10), strings.Repeat("a", 100)},
		"100 caracteres multibyte":        {strings.Repeat("\u00e1", 100), strings.Repeat("\u00e1", 100)},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			got, err := cimd.SanitizeClientName(caso.bruto)

			require.NoError(t, err)
			assert.Equal(t, caso.esperado, got)
		})
	}
}

func TestSanitizeClientName_NomeInvalido_RecusaComMensagemASCII(t *testing.T) {
	casos := map[string]string{
		"vazio":                               "",
		"so espacos e invisiveis":             " \t\n\u200b ",
		"UTF-8 invalido":                      "Agente \xff",
		"101 caracteres":                      strings.Repeat("a", 101),
		"zalgo":                               "Z\u0351\u0352\u0353a\u0354\u0355\u0356l\u0357g\u0358o",
		"tres marcas seguidas":                "a\u0301\u0302\u0303",
		"marcas juntadas ao tirar formatacao": "a\u0301\u200b\u0302\u0303",
	}
	for nome, bruto := range casos {
		t.Run(nome, func(t *testing.T) {
			_, err := cimd.SanitizeClientName(bruto)

			require.Error(t, err)
			for _, r := range err.Error() {
				assert.True(t, r >= 0x20 && r <= 0x7e, "mensagem com %q fora do ASCII visivel: %s", r, err)
			}
		})
	}
}
