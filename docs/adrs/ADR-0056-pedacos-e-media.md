# ADR-0056: Pedaços e média

## Status

Proposta

## Contexto

O modelo de embedding lê uma janela limitada de tokens, e a nota pode ser maior que ela. O kNN
da ligação por sentido (ADR-0053) precisa de um vetor por nota

## Decisão

O indexador de embedding corta a nota em pedaços de até 512 tokens, configurável, nas fronteiras
do tokenizador do modelo, e fica com os 8 primeiros. No provedor remoto, que não expõe o
tokenizador, um token vale 4 letras. O vetor da nota é a média dos vetores normalizados dos
pedaços, normalizada de novo

O modelo local é o granite-embedding-311m-multilingual-r2, fixado numa revisão, com o vetor do
token CLS e sem prefixo no texto. Em x86 com AVX2 ele usa os pesos
quantizados, e nas outras CPUs os cheios. Os arquivos são baixados quando a instalação liga o
provedor local, e o arquivo com outro hash é recusado

## Alternativas consideradas

- Só o primeiro pedaço: o resto da nota não conta
- Um vetor por pedaço, com kNN entre pedaços: multiplica os vetores e liga notas por um parágrafo
- Máximo por dimensão no lugar da média: puxa o vetor para o pedaço mais extremo

## Consequências

- O texto depois do oitavo pedaço não entra no vetor
- A nota longa que trata de vários assuntos vira um vetor médio, mais fraco em cada um
- Mudar o tamanho do pedaço não refaz sozinho os vetores já gravados
- Sem AVX2, o modelo local baixa e roda os pesos cheios, quatro vezes maiores
