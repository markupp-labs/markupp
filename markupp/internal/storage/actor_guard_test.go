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
// server OAuth usa tabelas da instância (ADR-0046), os segredos gerados na primeira
// subida também são da instância (ADR-0041), e os outros
// abrem o pool, migram e montam a transação do ator. A leitura sem conta (ADR-0048) é a
// segunda entrada aceita: passa pela transação pública, e só no repositório público.
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
	"oauth_cimd.go":          true,
	"oauth_dcr.go":           true,
	"instance_secrets.go":    true,
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

// arquivosDaLeituraPublica são os únicos que entram na transação do papel
// markupp_reader: quem a define e o repositório da leitura sem conta.
var arquivosDaLeituraPublica = map[string]bool{
	"actor_tx.go":      true,
	"public_vaults.go": true,
}

var entradaPublica = regexp.MustCompile(`\binPublicTx\(`)

func TestTransacaoPublica_SoNoRepositorioDaLeituraSemConta(t *testing.T) {
	arquivos, err := filepath.Glob("*.go")
	require.NoError(t, err)

	usos := []string{}
	for _, nome := range arquivos {
		if strings.HasSuffix(nome, "_test.go") {
			continue
		}
		fonte, err := os.ReadFile(nome) // #nosec G304 -- arquivo do proprio pacote, listado pelo Glob
		require.NoError(t, err)
		if entradaPublica.Match(fonte) {
			usos = append(usos, nome)
			assert.True(t, arquivosDaLeituraPublica[nome], "%s entra no papel markupp_reader fora do repositorio publico", nome)
		}
	}
	assert.ElementsMatch(t, []string{"actor_tx.go", "public_vaults.go"}, usos, "a transacao publica precisa existir e ser usada")
}
