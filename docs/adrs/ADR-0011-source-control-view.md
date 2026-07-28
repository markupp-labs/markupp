# ADR-0011: Source Control View no plugin, no lugar dos comandos avulsos

## Status

Aceita

## Contexto

A primeira versão da sincronização do plugin era um conjunto de comandos avulsos em
`src/commands/` (`sync`, `sync-all`, `upload`, `import`, `download`). Cada comando
reimplementava partes do mesmo fluxo (buscar remoto -> diferenciar -> aplicar ->
persistir meta), o que gerava duplicação, dificultava enxergar o estado de sincronização
e não dava ao usuário uma visão do que iria subir/descer antes de agir.

## Decisão

Consolidar a lógica numa camada de orquestração (`src/core/operations.ts`) sobre um motor
de status (`src/core/status.ts`), e expor isso por uma **Source Control View** estilo git
(`src/ui/sourceControl/view.ts`), com as operações `fetch`, `pull`, `push` e `sync`. O par
de persistência de meta (`setNoteMeta` + `syncRemoteSnapshot`), antes repetido em cada
`apply*`, foi extraído para o helper único `recordSynced`. `forcePush`/`forcePull` deixam de
reimplementar create/update/delete e passam a reaproveitar os mesmos `apply*` com um
parâmetro `force`.

## Alternativas consideradas

- Manter os comandos avulsos (status quo): cada comando duplica o fluxo e não há uma visão
  única de status; evoluir a sincronização exige mexer em vários arquivos paralelos.
- Só extrair helpers, sem a view: reduz duplicação mas não resolve a falta de visibilidade
  do estado para o usuário.

## Consequências

- Uma única camada (`core/`) concentra a lógica de sincronização, consumida pela view e
  pelos comandos finos do `main.ts`.
- O usuário passa a ver o diff (novo/modificado/deletado/conflito) antes de aplicar.
- Push/pull ficaram resilientes a falha parcial e a conflito 409 (optimistic locking):
  o `saveData` num `finally` e o conflito contado como `skipped` em vez de abortar o lote.
- Menos duplicação, porém a camada `core/` concentra responsabilidade e precisa de bons testes
  (cobertos em `operations.test.ts` e `status.test.ts`).

## Métrica antes/depois (reengenharia — Entrega 8)

Medido entre o ponto de partida da branch (`76f3145`) e o estado final do PR #79 (`0299ec3`).

| Métrica | Antes | Depois |
|---|---|---|
| Módulos de comando com lógica de sync duplicada (`src/commands/*.ts`) | 5 arquivos (413 LOC) | 0 (removidos) |
| Camada que concentra a sincronização | dispersa nos 5 comandos | `core/operations.ts` (384) + `core/status.ts` (108) |
| Repetição do par persistir-meta (`setNoteMeta`+`syncRemoteSnapshot`) inline | 5 pontos | 1 helper `recordSynced` (4 chamadas) |
| Reimplementação de create/update/delete em `forcePush` | sim | reaproveita `apply*` com parâmetro `force` |
| Visão de status para o usuário antes de aplicar | nenhuma | Source Control View (diff novo/modificado/deletado/conflito) |

**Análise:** não é uma redução bruta de linhas — é reorganização para coesão e remoção de
duplicação. A lógica de sincronização, antes espalhada e repetida em 5 comandos, passou a ter
uma fonte única (`core/`), o ponto de persistência de meta deixou de ser copiado em cada
caminho (5 -> 1), e `forcePush` parou de duplicar o CRUD. O ganho é de manutenibilidade e de
visibilidade do estado, não de tamanho do código.
