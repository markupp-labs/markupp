# ADR-0007: API REST como única interface de acesso

## Status

Aceita

## Contexto

Clientes (Obsidian, agentes de IA) podem estar em máquinas diferentes do servidor e precisam ler e escrever documentos

## Decisão

Toda interação com o servidor passa pela API REST. Cliente nunca acessa o armazenamento diretamente

## Alternativas consideradas

- Cliente acessando diretamente o armazenamento do servidor: acopla o cliente à infra e quebra com acesso remoto

## Consequências

- Obsidian exigirá plugin próprio para falar com o servidor
