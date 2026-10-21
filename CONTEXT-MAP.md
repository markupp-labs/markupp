# Context Map

## Contextos

- [Markupp](./markupp/CONTEXT.md), o servidor Go que guarda as notas, deriva a organização e
  expõe a API REST

## Relações

Contexto único por enquanto. O painel web vive em `web/`, neste repositório, e o plugin do
Obsidian vive em `markupp-labs/obsidian-markupp-plugin`. Os dois são clientes da API REST
deste contexto e não alcançam o banco (ADR-0026, ADR-0030, ADR-0031).
