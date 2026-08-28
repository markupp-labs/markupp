# ADR-0026: API REST como contrato público versionado

## Status

Aceita

## Contexto

O plugin do Obsidian deixou de ser a única interface prevista. O painel web, o servidor MCP e
clientes escritos por terceiros consomem a mesma API. Hoje as rotas não têm prefixo de versão, e
a especificação OpenAPI cobre 4 das 6 rotas existentes

## Decisão

A API REST é contrato público, versionada por prefixo de caminho a partir de `/v1`. Dentro de
uma versão não entra mudança que quebre cliente. Todos os clientes são pares sobre essa API,
escritos por nós ou por terceiros, e o plugin do Obsidian é o primeiro deles

## Alternativas consideradas

- Documentar sem versionar: menos cerimônia agora, e qualquer evolução quebra cliente de
  terceiro sem aviso
- Não assumir compromisso de estabilidade: quem escrever cliente assume o risco, o que na
  prática impede que alguém escreva
- Versionar por cabeçalho em vez de caminho: mais elegante e menos visível, e atrapalha testar
  uma rota no navegador ou no curl

## Consequências

- As rotas sem prefixo continuam servidas durante uma transição, marcadas como obsoletas, para
  não quebrar a instalação da `v1.0.0-rc.1` já publicada
- Completar a especificação OpenAPI, hoje sem `GET /notes` e `GET /notes/search`, deixa de ser
  capricho e vira pré-requisito
- O servidor MCP e o painel web são clientes da mesma API e não ganham acesso privilegiado ao
  armazenamento, o que reforça o ADR-0007
- Manter a versão anterior viva durante a transição custa código e teste
- Como cada cliente representa cofre, pasta e sincronização é decisão do cliente, não da API
