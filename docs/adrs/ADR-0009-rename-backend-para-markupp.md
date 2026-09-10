# ADR-0009: Renomear `backend` para `markupp`

## Status

Aceita

## Contexto

A pasta do servidor chamava `backend`, o que tratava o produto como acessório de um frontend. O servidor é o produto central e self-hosted

## Decisão

Renomear a pasta `backend` para `markupp` e `backendUrl` para `serverUrl` no plugin

## Alternativas consideradas

- Manter `backend`: menos churn, mas reforça a leitura de que o servidor é secundário
- Nome genérico `server`: claro, mas perde a identidade do produto

## Consequências

- O nome reflete que o servidor é o produto, não um anexo
- Ajustes em imports, Docker e CI
- Alinha o repositório ao discurso do MVP
