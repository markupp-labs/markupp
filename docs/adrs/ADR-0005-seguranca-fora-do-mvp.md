# ADR-0005: Segurança fora do MVP

## Status

Aceita

## Contexto

O MVP será demonstrado em rede interna isolada, sem exposição à internet

## Decisão

TLS, autenticação e rate limit ficam fora do MVP. A API escuta HTTP puro, sem auth

## Alternativas consideradas

- Token fixo simples: ainda exige configuração e não resolve TLS
- Adiar tudo para depois da primeira entrega: rejeitado, a decisão precisa ser rastreável

## Consequências

- Deploy trivial para a demo
- Sistema não pode ser exposto à internet até essas camadas entrarem
