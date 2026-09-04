# ADR-0028: Decomposição em serviços por perfil de carga

## Status

Aceita

## Contexto

Com Kubernetes (ADR-0027), cada serviço escala e falha por conta própria, e o dimensionamento
de recurso é por serviço. A tentação é dividir por entidade de domínio, o que coloca chamada de
rede e transação distribuída dentro do caminho de escrita de uma nota

## Decisão

A divisão segue perfil de recurso, não entidade de domínio:

- API REST: ligada à entrada e saída, carga constante, sempre ligada
- Servidor MCP: mesmo formato de requisição, tráfego de agente com rajada própria, isolado para
  não degradar a API usada por pessoas
- Indexador determinístico: CPU em rajada, sem modelo, escala a zero
- Indexador de embedding: perfil de recurso distinto, com acelerador quando houver modelo
  local, escala a zero
- Worker de notificação: rajada com repetição, fora do caminho da requisição
- Plano de controle: cobrança, provisionamento e plano, tráfego baixo e postura de segurança
  distinta
- Painel web: arquivo estático em armazenamento de objeto com CDN, sem processo próprio

Nota, cofre, usuário e busca não são serviços: compartilham banco e caminho de requisição

## Alternativas consideradas

- Divisão por entidade de domínio, com serviços de notas, cofres, usuários e busca: é a leitura
  literal de microsserviços, e transforma uma escrita de nota em várias chamadas de rede com
  transação distribuída
- Três serviços, com API, indexador e plano de controle: menos para operar, e junta perfis
  distintos, como embedding com acelerador e indexação determinística barata, no mesmo
  dimensionamento
- Monolito com réplicas: o mais simples de operar, e obriga dimensionar tudo pelo pico do
  componente mais pesado

## Consequências

- Cada serviço recebe requisição de recurso própria, e o acelerador só é consumido enquanto o
  indexador de embedding roda
- A escrita de uma nota continua sendo uma transação em um banco, sem salto de rede
- Sete artefatos para construir, versionar e implantar, contra um hoje
- O painel não consome computação, então o custo dele é armazenamento e transferência
- Os serviços compartilham o mesmo banco, então o isolamento entre eles é de processo e de
  escala, não de dado: não são microsserviços no sentido de banco por serviço
