# MVP - Markupp

## Objetivo

Validar a base da solução: um hub centralizado onde o usuário consegue criar, ler, editar e excluir anotações em markdown a partir de um cliente

## Funcionalidades essenciais

- Criar arquivos markdown
- Visualizar arquivos markdown
- Editar arquivos markdown
- Excluir arquivos markdown
- Renomear arquivos markdown
- API REST para operações sobre os documentos
- Um cliente de referência consumindo a API
- Armazenamento centralizado dos arquivos

## Fora do escopo do MVP

- Busca semântica e léxica
- Indexação vetorial
- Taxonomias e classificação
- Organização hierárquica (pastas)
- Colaboração em tempo real

## Por que esse recorte é viável

- O CRUD pela API com um cliente de referência é um escopo contido que 4 pessoas conseguem entregar no semestre
- Não depende de integrações complexas (IA, embeddings, banco vetorial)

## Critérios de decisão

- Entra no MVP o que é necessário para o usuário conseguir usar o sistema de forma básica (criar, ver, editar, renomear e apagar anotações)
- Fica de fora o que agrega valor mas depende do CRUD funcionando primeiro (busca, taxonomia, organização)
