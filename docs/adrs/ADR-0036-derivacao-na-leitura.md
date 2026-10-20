# ADR-0036: Sugestão e menção derivadas na leitura

## Status

Aceita

## Contexto

Sugestão de ligação e menção não ligada dependem do cofre inteiro: uma nota nova cria
menções em notas antigas que não estão no lote do indexador. A premissa é manter o indexador
leve (ADR-0033), com custo proporcional ao que mudou

## Decisão

Sugestão e menção são calculadas por nota, na leitura, sobre o que o indexador já grava: o
índice léxico, os títulos, as ligações e, para a sugestão, as postagens tf-idf de cada nota
(termos, pesos e norma do vetor). O cosseno da leitura lê só essas postagens. Nenhuma
sugestão nem menção é gravada

## Alternativas consideradas

- Tabela de menções e sugestões gravada pelo indexador: leitura mais barata, mas cada título
  novo exige revarrer o cofre inteiro para achar quem o menciona

## Consequências

- O lote do indexador paga só as postagens das notas dele, com o df vigente
- A resposta reflete os títulos e as ligações do momento, sem esperar reindexação
- O custo vai para a leitura
- Título em mais de max(5% das notas do cofre, 5) notas é palavra comum e não vira menção
  (keyphraseness, Mihalcea e Csomai, 2007); o piso de cinco notas preserva as menções do
  cofre pequeno, e cofre ainda sem df não filtra
