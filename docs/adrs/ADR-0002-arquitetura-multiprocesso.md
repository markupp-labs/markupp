# ADR-0002: Arquitetura multiprocesso

## Status

Aceita, com duas partes substituídas

A divisão em processos independentes segue valendo. A escolha de Rust ou Python para
indexação e busca semântica foi substituída pelo ADR-0031, que fixa Go nos dois indexadores.
A alternativa "microserviços via rede", descartada aqui, foi retomada pelos ADR-0027 e
ADR-0028, que separam sete serviços com ciclo de vida próprio sobre Kubernetes

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
