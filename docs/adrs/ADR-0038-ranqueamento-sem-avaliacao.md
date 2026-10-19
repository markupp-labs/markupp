# ADR-0038: Ranqueamento da sugestão escolhido pela literatura

## Status

Aceita

## Contexto

A sugestão de ligação precisa ordenar candidatas por dois sinais, a estrutura das ligações e
o texto. Escolher medida e combinação por avaliação, escondendo ligações e medindo Hits@k e
MRR, pede protocolo, cofres de teste e uma verdade de referência que um cofre pessoal não
tem

## Decisão

A lista estrutural usa alocação de recursos na vizinhança de dois saltos (Zhou, Lü e Zhang,
2009), a lista léxica usa cosseno tf-idf sobre os 50 termos de maior peso de cada nota, com
candidatas só entre as notas que têm algum dos 10 termos de maior peso da nota, e as duas
são fundidas por Reciprocal Rank Fusion com k=60 (Cormack, Clarke e Büttcher, 2009).
Os parâmetros são os canônicos dos artigos, fixos no código. Não há avaliação por Hits@k
nem MRR

## Alternativas consideradas

- Avaliação por ligações escondidas: mede, mas cada métrica é limite inferior num mundo
  aberto e favorece as medidas de vizinho comum
- Vizinhos em comum ou Adamic-Adar no lugar da alocação de recursos: favorecem notas-índice
  ou descontam menos o intermediário muito ligado
- Combinação linear dos dois sinais: pede escala comum e um peso que só uma avaliação
  escolheria

## Consequências

- Nenhum parâmetro a calibrar por instalação
- O corte em 10 termos limita as candidatas às notas que dividem com a nota os termos
  raros dela; a nota parecida só por termos comuns fica de fora
- A qualidade da sugestão não é medida; a escolha se apoia nos artigos
- Trocar a medida ou acrescentar um sinal, como o vetor do indexador de embedding, é
  acrescentar uma lista à fusão
