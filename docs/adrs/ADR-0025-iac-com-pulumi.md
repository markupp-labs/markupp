# ADR-0025: Infraestrutura como código com Pulumi em Go

## Status

Aceita

## Contexto

O Enterprise roda em AWS, com Lambda para indexação, banco gerenciado, armazenamento de objeto,
envio de email e balanceador. Nada disso está descrito em código hoje, e a implantação precisa
ser reproduzível

## Decisão

Pulumi, com os programas escritos em Go, cobrindo apenas a infraestrutura do Enterprise. O
estado fica em bucket S3 da própria conta, não no Pulumi Cloud

## Alternativas consideradas

- Terraform: mais difundido e com estado explícito, e descreve a infraestrutura em linguagem
  própria, separada da linguagem da aplicação
- AWS CDK: o binding de Go é o mais fraco dos disponíveis, porque a ergonomia foi desenhada
  para TypeScript
- AWS SAM: cobre bem a parte serverless, e a pilha tem banco, armazenamento, email e
  balanceador além do Lambda
- Estado no Pulumi Cloud: menos infraestrutura para montar e histórico pronto, e é gratuito só
  para uso individual, além de tirar da conta um arquivo que guarda identificador de cliente
- Publicar módulo para quem hospeda por conta própria: significaria dar suporte a conta, região
  e variação de infraestrutura de terceiros, sem receita associada

## Consequências

- A infraestrutura fica na mesma linguagem do servidor, com o mesmo lint e a mesma forma de
  testar
- O bucket de estado precisa existir antes da primeira execução, então há um passo manual de
  origem que não pode ser descrito pelo próprio Pulumi
- O bloqueio de estado precisa estar habilitado no backend, senão duas execuções simultâneas
  corrompem o estado
- Quem hospeda por conta própria continua recebendo o compose, sem IaC
