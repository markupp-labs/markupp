# ADR-0003: SQLite como banco de dados

## Status

Aceita

## Contexto

Precisamos de um banco relacional leve que não exija infraestrutura extra do usuário

## Decisão

SQLite, tanto para dados relacionais quanto para banco vetorial

## Alternativas consideradas

- PostgreSQL: exige servidor separado + pesado

## Consequências

- Banco é um arquivo, zero configuração para o usuário
- Suporte vetorial
