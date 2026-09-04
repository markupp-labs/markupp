# ADR-0027: Kubernetes como plataforma de execução

## Status

Aceita

## Contexto

O Enterprise foi desenhado sobre Lambda e balanceador da AWS (ADR-0018, ADR-0023), enquanto o
self-host roda por compose. São dois runtimes diferentes para o mesmo produto, e o serverless
amarra o projeto a uma nuvem. Além disso, self-host é a identidade do projeto: o que a equipe
opera na nuvem deveria ser o que qualquer pessoa consegue operar

## Decisão

Kubernetes é a plataforma de execução do Enterprise e do self-host de porte maior, com as
mesmas imagens de container usadas no compose. A indexação roda como Job com escala a zero, e
o TLS termina no ingress. O compose continua sendo a via para instalação de um nó

## Alternativas consideradas

- Manter Lambda e balanceador da AWS: mais barato em volume baixo e sem cluster para operar, ao
  custo de duas arquiteturas, de amarra a uma nuvem, e da taxa que o Lambda cobra em pool de
  conexão obrigatório, teto de quinze minutos e cold start
- Só Kubernetes, sem compose: uma forma de implantar e menos para manter, e afasta o
  self-hoster individual, que é justamente quem roda compose
- Nomad ou Docker Swarm: menos operação que Kubernetes, com ecossistema e disponibilidade de
  mão de obra muito menores

## Consequências

- O que a equipe opera na nuvem passa a ser uma instalação que qualquer pessoa reproduz
- Some a taxa do Lambda: sem pool obrigatório na frente do Postgres, sem teto de quinze minutos
  e sem fatiar lote por causa de tempo
- A indexação pode rodar em nó interrompível, porque o plano de recuperação é descartável por
  decisão (ADR-0012): perder o nó no meio custa reprocessar, não perder dado
- Substitui o ADR-0018 e o ADR-0023. O self-host de um nó continua terminando TLS no próprio
  servidor com Let's Encrypt, como o ADR-0023 definia
- Operar Kubernetes é trabalho novo para uma equipe pequena, e o cluster tem piso de custo que
  o Lambda não tinha
- Portabilidade não sai de graça: ingress, classe de armazenamento e identidade ainda têm
  arestas específicas de cada nuvem
