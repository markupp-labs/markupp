# Qualidade

## Atributos priorizados

- **Manutenibilidade**: o escopo cresce e muda. O código precisa aceitar requisito novo sem acumular débito.
- **Compatibilidade**: quem faz self-host sobe o markupp na própria máquina. As imagens saem para linux/amd64 e linux/arm64.
- **Confiabilidade**: o markupp guarda as notas de quem usa. Perda ou corrupção de dados e edição perdida são as falhas mais graves.
- **Desempenho**: servidor e clientes não devem gastar memória e CPU sem necessidade. Nenhuma verificação automática cobre esse atributo, ele fica a cargo da revisão.

## O que a CI verifica

O [workflow de CI](../.github/workflows/ci.yml) roda em cada PR e a cada push na main, e cada job só roda quando o seu pacote muda.

| Pacote | Verificações |
|---|---|
| Servidor Go (markupp/) | gofmt, go vet, golangci-lint, testes com -race, cobertura mínima de 90% (sem código gerado), govulncheck |
| Servidor MCP (mcp/) | prettier, tsc, vitest, instalação do pacote empacotado pelo npx |
| Componente Pulumi (deploy/pulumi/) | go vet, golangci-lint, testes com -race |
| Painel (web/) | rubocop, brakeman, bundler-audit, importmap audit, testes do Rails |
| Todos | testes dos scripts de apoio e validação das mensagens de commit do PR |

Dois workflows rodam em PRs que tocam as partes envolvidas e a cada push na dev:

- [e2e-mcp.yml](../.github/workflows/e2e-mcp.yml) sobe o compose de desenvolvimento e testa o servidor MCP de ponta a ponta contra o servidor Go, pelos dois transportes.
- [smoke-dev.yml](../.github/workflows/smoke-dev.yml) sobe a stack dev do Pulumi num cluster kind e testa a API por https pelo Gateway.

Os mesmos testes rodam localmente com `make e2e-mcp`, `make smoke-kind` e `make coverage`.

## Hooks locais

`make hooks` instala os hooks do [.pre-commit-config.yaml](../.pre-commit-config.yaml):

- commit-msg: valida a mensagem de commit (ver [fluxo de trabalho](fluxo-de-trabalho.md#commits)).
- pre-commit: goimports nos arquivos Go alterados.
- pre-push: govulncheck nos módulos Go, quando o push mexe em código Go.
