# ADR-0015: PostgreSQL em toda instalação, no lugar do SQLite

## Status

Aceita

## Contexto

Uma instalação em nuvem vai rodar como instância única, com divisão lógica por tenant, e a
indexação vai rodar sob demanda em processo separado. SQLite é embarcado e de escritor
único: dois processos não escrevem o mesmo arquivo com segurança fora do mesmo host, e o
travamento dele não sobrevive a filesystem de rede

## Decisão

PostgreSQL em toda instalação, com compose ou em nuvem. O `sqlc` passa a gerar para Postgres e
o self-host recebe o banco como serviço no compose

## Alternativas consideradas

- Manter SQLite em toda instalação: impede indexação em processo separado, que é justamente o
  objetivo
- SQLite no self-host e Postgres na nuvem: dois dialetos de query para sempre, busca léxica e
  vetorial escritas duas vezes, e divergência de comportamento entre instalações que só aparece
  em produção de um dos lados

## Consequências

- Substitui o ADR-0003 e o ADR-0006
- Backup deixa de ser copiar um arquivo e passa a ser `pg_dump`
- O self-host ganha um container a mais no compose, prática já comum em software
  self-hosted como Immich, Paperless-ngx e Gitea
- Vetorial passa a ser pgvector, e busca léxica deixa de poder usar FTS5
- Uma implementação de query, de busca e de vetorial serve toda instalação

## Emenda

Em novembro de 2026, o texto passou a descrever a capacidade da instalação no lugar de uma
edição. A decisão não mudou.
