# Testes de Aceitação (MVP)

Os testes de aceitação do MVP são automatizados e rodam no CI. Cobrem as
funcionalidades principais (RF1 a RF5), listagem, busca e conflito. O fluxo
dentro do Obsidian é validado manualmente.

## Cobertura

| Funcionalidade (RF / issue) | Onde é testado |
| --- | --- |
| Criar e ler nota (RF1/RF2, #2/#1/#16) | `markupp/internal/api/integration_test.go` |
| Listar notas (#67) | idem |
| Editar e renomear (RF5/RF4, #7/#6) | idem |
| Excluir nota (RF3, #5) | idem |
| Conflito 409 e force (#59) | idem |
| Operações do plugin: fetch/pull/push/sync (#57/#68/#78) | `src/core/*.test.ts` no [repositório do plugin](https://github.com/markupp-labs/obsidian-markupp-plugin) |

CI: job "Servidor (Go) tests" aqui, "Obsidian plugin build" no repositório do plugin. Fora do escopo do MVP:
ver [requisitos do MVP](requisitos-mvp.md).

## Roteiro manual (Obsidian)

Servidor no ar ([DEPLOY](DEPLOY.md)) e plugin configurado:

1. Criar nota, Push, conferir em `GET /notes`
2. Editar, Sync, conferir o conteúdo
3. Renomear, Push, conferir o `path`
4. Excluir, Push, conferir a remoção
5. Fetch e Pull em um vault limpo
6. Editar dos dois lados, conferir o conflito na Source Control View e resolver
   com force
