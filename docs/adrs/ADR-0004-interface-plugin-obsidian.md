# ADR-0004: Plugin do Obsidian como interface

## Status

Aceita, com escopo limitado ao MVP

Valia enquanto o Obsidian era o único cliente previsto. O ADR-0026 estabelece que a interface é
a API REST e que os clientes são pares, sendo o plugin do Obsidian o primeiro deles

## Contexto

O usuário precisa de uma interface para interagir com seus documentos

## Decisão

Plugin do Obsidian escrito em TypeScript. O Obsidian hospeda a interface e consome a API REST do servidor

## Alternativas consideradas

- Aplicação web com React ou Rails: exigiria reconstruir editor e navegação que o Obsidian já oferece
- Electron/Tauri: mais complexo de distribuir, sem ganho para o MVP
- Mobile nativo: fora do escopo

## Consequências

- Reaproveita editor e navegação do Obsidian, reduzindo o escopo de UI
- Cria dependência do Obsidian
