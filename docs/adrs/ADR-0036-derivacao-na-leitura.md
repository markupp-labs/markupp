# ADR-0036: Sugestão e menção derivadas na leitura

## Status

Aceita

## Contexto

Sugestão de ligação e menção não ligada dependem do cofre inteiro: uma nota nova cria
menções em notas antigas que não estão no lote do indexador. A premissa é manter o indexador
leve (ADR-0033), com custo proporcional ao que mudou

## Decisão

Sugestão e menção são calculadas por nota, na leitura, sobre o que o indexador já grava: o
índice léxico, os títulos e as ligações. O indexador não ganha tabela nem passo para elas

## Alternativas consideradas

- Tabela de menções e sugestões gravada pelo indexador: leitura mais barata, mas cada título
  novo exige revarrer o cofre inteiro para achar quem o menciona

## Consequências

- O lote do indexador não muda de custo
- A resposta reflete os títulos e as ligações do momento, sem esperar reindexação
- O custo vai para a leitura, medido no benchmark do cofre sintético
