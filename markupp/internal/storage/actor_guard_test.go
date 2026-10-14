package storage

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// arquivosForaDoAtor podem usar o pool direto: autenticação roda antes de existir
// tenant (ADR-0034), e os outros abrem o pool, migram e montam a transação do ator.
var arquivosForaDoAtor = map[string]bool{
	"auth.go":     true,
	"actor_tx.go": true,
	"postgres.go": true,
}

var acessoDiretoAoPool = regexp.MustCompile(`gen\.New\(|pool\.(Begin|Exec|Query|QueryRow|SendBatch)\(`)

func TestRepositoriosDeDados_SoAcessamOBancoPelaTransacaoDoAtor(t *testing.T) {
	arquivos, err := filepath.Glob("*.go")
	require.NoError(t, err)

	for _, nome := range arquivos {
		if strings.HasSuffix(nome, "_test.go") || arquivosForaDoAtor[nome] {
			continue
		}
		fonte, err := os.ReadFile(nome)
		require.NoError(t, err)
		assert.Empty(t, acessoDiretoAoPool.FindAllString(string(fonte), -1),
			"%s acessa o pool sem inActorTx e roda como superusuario, fora do RLS", nome)
	}
}
