# ADR-0015: PostgreSQL nas duas edições, no lugar do SQLite

## Status

Aceita

## Contexto

A edição Enterprise vai rodar como instância única em nuvem, com divisão lógica por cliente,
e a indexação vai rodar sob demanda em processo separado. SQLite é embarcado e de escritor
único: dois processos não escrevem o mesmo arquivo com segurança fora do mesmo host, e o
travamento dele não sobrevive a filesystem de rede

## Decisão

PostgreSQL nas duas edições, self-host e Enterprise. O `sqlc` passa a gerar para Postgres e o
self-host recebe o banco como serviço no compose

## Alternativas consideradas

- Manter SQLite nas duas edições: impede indexação em processo separado, que é justamente o
  objetivo
- SQLite no self-host e Postgres na nuvem: dois dialetos de query para sempre, busca léxica e
  vetorial escritas duas vezes, e divergência de comportamento entre edições que só aparece
  em produção de um dos lados

## Consequências

- Substitui o ADR-0003 e o ADR-0006
- Backup deixa de ser copiar um arquivo e passa a ser `pg_dump`
- O self-host ganha um container a mais no compose, prática já comum em software
  self-hosted como Immich, Paperless-ngx e Gitea
- Vetorial passa a ser pgvector, e busca léxica deixa de poder usar FTS5
- Uma implementação de query, de busca e de vetorial serve as duas edições
