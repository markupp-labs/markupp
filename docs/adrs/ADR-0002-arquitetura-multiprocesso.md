# ADR-0002: Arquitetura multiprocesso

## Status

Aceita, com ressalvas

A divisão em processos independentes segue valendo. A alternativa "microserviços via rede",
descartada aqui, foi retomada pelos ADR-0027 e ADR-0028, que separam sete serviços com ciclo
de vida próprio sobre Kubernetes.

A linguagem por serviço fica em aberto de propósito. Rust ou Python para indexação é escolha
de antes do MVP, e o ADR-0028 e o ADR-0029 descrevem os indexadores em Go sem decidir nada.
Fica para quando o indexador for escrito

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
