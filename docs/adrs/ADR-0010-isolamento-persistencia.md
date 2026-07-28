# ADR-0010: Detalhes de persistência isolados na camada de storage

## Status

Aceita

## Contexto

As camadas api -> notes -> storage existem, mas detalhes de persistência vazam para
cima: api e notes importam database/sql e tratam sql.ErrNoRows, e o domínio monta a
sintaxe de LIKE do SQL

## Decisão

A camada de storage é a única que conhece database/sql e SQL. Ela traduz erros de
persistência para erros de domínio (sql.ErrNoRows vira notes.ErrNotFound) e monta as
queries. As camadas acima falam apenas erros de domínio

## Alternativas consideradas

- Manter o tratamento de sql.ErrNoRows na API (status quo): acopla a API ao driver,
  trocar o storage exigiria mexer na API

## Consequências

- API e domínio independentes do driver de persistência
- Trocar o storage não afeta as camadas superiores
- Erros de domínio são o contrato único entre camadas
- O storage precisa traduzir os erros de persistência explicitamente
