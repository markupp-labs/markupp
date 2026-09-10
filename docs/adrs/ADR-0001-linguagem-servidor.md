# ADR-0001: Go como linguagem do servidor

## Status

Aceita

## Contexto

Precisamos de uma linguagem para a API REST e o núcleo do sistema

## Decisão

Go.

## Alternativas consideradas

- Python: familiaridade, mas performance inferior para servir a API
- Java: robusto e com familiaridade, mas pesado demais para um sistema self-hosted

## Consequências

- Binário único, deploy simples
- Boa performance com baixo consumo de recursos
- Elegante
