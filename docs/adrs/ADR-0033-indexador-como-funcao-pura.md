# ADR-0033: Indexador como função pura

## Status

Aceita

## Contexto

O ADR-0027 substituiu o ADR-0018 inteiro, e com ele foi embora uma decisão que não dependia do
Lambda: o indexador não sabe quem o invocou. Com Job no cluster, laço no compose e KEDA
disparando (ADR-0032), existem várias formas de invocar a mesma indexação

## Decisão

O indexador é uma função de (tenant, conjunto de mudanças) para linhas derivadas, sem estado
próprio e sem saber quem o invocou. O conjunto de mudanças vem do cursor de revisão. Cada forma
de invocação é um entrypoint fino sobre o mesmo pacote: execução única no Job do cluster e laço
no compose

## Alternativas consideradas

- Indexador ciente da plataforma, lendo variáveis do Job ou falando com a API do Kubernetes:
  menos camadas, e cada edição passa a ter uma indexação própria
- Deixar a pureza implícita, sem registro: nada impede o próximo entrypoint de carregar estado
  para dentro do pacote

## Consequências

- Uma implementação de indexação serve o cluster e o compose, sem edição com funcionalidade a
  menos
- O indexador é testável sem Kubernetes, com um conjunto de mudanças montado no teste
- Trocar a forma de disparo, de KEDA para CronJob ou para broker, não toca o pacote de indexação
- Indexação atrasada não impede leitura nem escrita de nota, como o ADR-0012 prometeu
