# Context Map

## Contextos

- [Markupp](./markupp/CONTEXT.md), o servidor Go que guarda as notas, deriva a organização e
  expõe a API REST

## Relações

Contexto único por enquanto. O plugin do Obsidian vive em
`markupp-labs/obsidian-markupp-plugin` e é um cliente da API REST deste contexto (ADR-0026,
ADR-0030).
