# ADR-0027: Kubernetes como plataforma de execução

## Status

Aceita, alterada pelo ADR-0032

## Contexto

O Enterprise foi desenhado sobre Lambda e balanceador da AWS (ADR-0018, ADR-0023), enquanto o
self-host roda por compose. São dois runtimes diferentes para o mesmo produto, e o serverless
amarra o projeto a uma nuvem. Além disso, self-host é a identidade do projeto: o que a equipe
opera na nuvem deveria ser o que qualquer pessoa consegue operar

## Decisão

Kubernetes é a plataforma de execução do Enterprise e do self-host de porte maior, com as
mesmas imagens de container usadas no compose. A indexação roda como Job criado sob demanda
(ADR-0032), e o TLS termina no Gateway. O compose continua sendo a via para instalação de um nó

A implantação vem em duas fases. Na primeira, o Enterprise roda no cluster kubeadm do campus
São José do IFSC, com Cilium, Gateway API, cert-manager e Longhorn. Na segunda, vai para nuvem
pública, com a mesma instalação. O PostgreSQL roda numa instância só, dentro do namespace do
markupp, sem nada acrescentado no nível do cluster para ele

## Alternativas consideradas

- Manter Lambda e balanceador da AWS: mais barato em volume baixo e sem cluster para operar, ao
  custo de duas arquiteturas, de amarra a uma nuvem, e da taxa que o Lambda cobra em pool de
  conexão obrigatório, teto de quinze minutos e cold start
- Só Kubernetes, sem compose: uma forma de implantar e menos para manter, e afasta o
  self-hoster individual, que é justamente quem roda compose
- PostgreSQL com CloudNativePG em duas instâncias: failover automático e backup gerenciado, ao
  custo de CRD, webhook e operador no cluster compartilhado com outros projetos
- Nomad ou Docker Swarm: menos operação que Kubernetes, com ecossistema e disponibilidade de
  mão de obra muito menores

## Consequências

- O que a equipe opera passa a ser uma instalação que qualquer pessoa com Pulumi reproduz
  (ADR-0025). Quem usa Helm, Argo CD ou Flux fica sem artefato oficial
- Some a taxa do Lambda: sem pool obrigatório na frente do Postgres, sem teto de quinze minutos
  e sem fatiar lote por causa de tempo
- Substitui o ADR-0023 e a forma de invocação do ADR-0018. A indexação como função pura
  continua valendo, pelo ADR-0033. O self-host de um nó continua terminando TLS no próprio
  servidor com Let's Encrypt, como o ADR-0023 definia
- Operar Kubernetes é trabalho novo para uma equipe pequena, e o cluster tem piso de custo que
  o Lambda não tinha
- Na primeira fase o backup do banco é snapshot de volume no próprio cluster. Perder o cluster
  leva as notas junto
- O banco não tem failover: se o nó dele cair, a API fica sem banco até o pod subir em outro nó
- A entrada de tráfego usa Gateway API, e não Ingress. A API Ingress está congelada e a
  documentação do Kubernetes recomenda Gateway no lugar dela. São três recursos com donos
  distintos, GatewayClass, Gateway e HTTPRoute, e o cluster precisa dos CRDs e de um controlador
  que os implemente
- Portabilidade não sai de graça: gateway, classe de armazenamento e identidade ainda têm
  arestas específicas de cada nuvem
