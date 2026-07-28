# ADR-0002: Arquitetura multiprocesso

## Status

Substituída pelo ADR-0012

## Contexto

O sistema tem responsabilidades distintas que podem se beneficiar de linguagens diferentes

## Decisão

Cada serviço roda como processo independente na linguagem mais adequada:

- API e banco de dados: Go
- Indexação e busca semântica: Rust ou Python

## Alternativas consideradas

- Monolito: mais simples, mas limita a escolha de linguagem
- Microserviços via rede: self-hosting estranho

## Consequências

- Liberdade para usar a melhor ferramenta para cada tarefa
- Maior complexidade de deploy comparado a um monolito
