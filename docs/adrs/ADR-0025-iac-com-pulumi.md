# ADR-0025: Infraestrutura como código com Pulumi em Go

## Status

Aceita

## Contexto

O Enterprise roda em Kubernetes (ADR-0027), primeiro num cluster on-premise e depois em nuvem
pública. A implantação precisa ser reproduzível pela equipe e por quem hospeda por conta
própria, e nada disso está descrito em código hoje

## Decisão

Pulumi, com um componente escrito em Go e mantido neste repositório, versionado com a mesma tag
das imagens. O componente declara os recursos Kubernetes do markupp com tipos do SDK, sem chart
Helm por baixo, e é o artefato de instalação em Kubernetes para terceiros. A stack da equipe
consome o mesmo componente

O estado fica num bucket do Garage, e os segredos da stack são cifrados por passphrase. O CI
implanta com uma credencial restrita ao namespace do markupp. Gateway API e cert-manager são
pré-requisitos do cluster, e KEDA é opcional, instalado à parte por quem administra o cluster

## Alternativas consideradas

- Helm chart publicado em OCI: é o formato que terceiros esperam, e obriga manter templates e
  schema de valores fora da linguagem do servidor
- Kustomize: YAML sem template, e parametrizar para terceiros vira patch
- Operator próprio: resolve ciclo de vida complexo, e os serviços não têm estado
- Terraform: mais difundido, e descreve a infraestrutura em linguagem própria
- Estado no Pulumi Cloud: menos infraestrutura para montar, e é gratuito só para uso individual
- Estado em bucket de nuvem: sobrevive à perda do cluster, e traz uma conta de nuvem só para isso
- Credencial de administrador do cluster no CI: o CI instalaria os operadores também, e um
  vazamento alcançaria os outros projetos do cluster

## Consequências

- A infraestrutura fica na mesma linguagem do servidor, com o mesmo lint e a mesma forma de
  testar
- Reproduzir a instalação exige Pulumi. O componente em Go é consumido de qualquer linguagem do
  Pulumi, inclusive YAML, e quem usa Helm, Argo CD ou Flux fica sem artefato oficial
- O Garage roda no cluster, fora de qualquer stack e sem cópia. Perder o cluster leva o estado
  junto, e a recuperação é por `pulumi import`
- O bucket de estado precisa existir antes da primeira execução, então há um passo manual de
  origem que não pode ser descrito pelo próprio Pulumi
- Sem KEDA, os indexadores rodam por CronJob. Sem URL de banco externo, o componente sobe um
  Postgres de uma instância no namespace, com senha gerada
- Quem hospeda num nó só continua recebendo o compose, sem IaC
