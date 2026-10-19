# ADR-0039: Ligação inferida no grafo

## Status

Aceita

## Contexto

O segundo estágio escolhe, para cada ligação não resolvida, a nota mais provável: título a
no máximo 0,2 de distância de Damerau-Levenshtein normalizada, desempatado por RRF entre
distância, probabilidade a priori da âncora e parentesco no grafo. É um palpite, e não há
avaliação que meça quantas vezes ele erra. Falta decidir se o palpite entra nas medidas do
grafo ou fica só para quem navega

## Decisão

A ligação inferida é gravada em note_links com o tipo inferred e entra em tudo que lê o
grafo: ligações de saída e de entrada, nota órfã, menção não ligada, sugestão, PageRank e
comunidades, com o mesmo peso da ligação escrita. As listas de ligações a mostram com o
tipo inferred, e a ligação não resolvida que a originou traz o palpite. Ela vive só no
plano de recuperação (ADR-0013) e é refeita a cada segundo estágio, antes do PageRank e das
comunidades

## Alternativas consideradas

- Só navegação: as listas de ligações mostram a inferida, as medidas ficam só com as
  escritas; o erro não se espalha, mas o erro de digitação continua deixando a nota fora da
  comunidade dela
- Peso menor nas medidas: atenua o palpite, mas pede um peso que só uma avaliação
  escolheria

## Consequências

- O erro de digitação deixa de isolar a nota no PageRank e nas comunidades
- O palpite errado também entra nas medidas, e a taxa de erro não é medida
- A ligação não resolvida continua listada como foi escrita, e o cliente vê o palpite marcado
