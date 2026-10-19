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
// tenant (ADR-0034), o indexador reserva o cofre antes de entrar no papel
// markupp_indexer, o notificador trabalha no papel markupp_notifier, o authorization
// server OAuth usa tabelas da instância (ADR-0039), e os outros
// abrem o pool, migram e montam a transação do ator.
var arquivosForaDoAtor = map[string]bool{
	"auth.go":                true,
	"actor_tx.go":            true,
	"postgres.go":            true,
	"vault_indexer.go":       true,
	"notification_outbox.go": true,
	"account_email.go":       true,
	"oauth.go":               true,
	"oauth_refresh.go":       true,
	"oauth_code.go":          true,
}

var acessoDiretoAoPool = regexp.MustCompile(`gen\.New\(|\binTx\(|pgx\.BeginFunc|pool\.\w+\(`)

func TestRepositoriosDeDados_SoAcessamOBancoPelaTransacaoDoAtor(t *testing.T) {
	arquivos, err := filepath.Glob("*.go")
	require.NoError(t, err)

	for _, nome := range arquivos {
		if strings.HasSuffix(nome, "_test.go") || arquivosForaDoAtor[nome] {
			continue
		}
		fonte, err := os.ReadFile(nome) // #nosec G304 -- arquivo do proprio pacote, listado pelo Glob
		require.NoError(t, err)
		assert.Empty(t, acessoDiretoAoPool.FindAllString(string(fonte), -1),
			"%s acessa o pool sem inActorTx e roda como superusuario, fora do RLS", nome)
	}
}
