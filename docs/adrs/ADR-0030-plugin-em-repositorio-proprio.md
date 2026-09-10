# ADR-0030: Plugin do Obsidian em repositório próprio

## Status

Aceita

## Contexto

O plugin morava em `obsidian-plugin/` no mesmo repositório do servidor. Os dois têm
toolchains, ciclos de release e públicos diferentes: o servidor entrega imagem Docker por
tag, o plugin entrega zip anexado à release. O ADR-0026 já estabelece que a interface do
sistema é a API REST e que os clientes são pares, então o plugin não tem posição
privilegiada que justifique dividir o repositório

## Decisão

O plugin passa a viver em `markupp-labs/obsidian-markupp-plugin`, com histórico preservado
via `git subtree split`. A documentação referente ao plugin acompanha o código. Este
repositório fica só com o servidor

## Alternativas consideradas

- Manter o monorepo: um clone e um PR cobrem mudanças que cruzam servidor e plugin, ao
  custo de CI que roda Go e Node em todo PR e de releases que carregam o versionamento um
  do outro
- Manter o diretório como submódulo: preserva o caminho `obsidian-plugin/`, e traz o custo
  conhecido de submódulo (clone em dois passos, ponteiro que desatualiza em silêncio)

## Consequências

- Cada repositório versiona e libera no próprio ritmo, sem tag do servidor arrastando o
  plugin
- O CI daqui roda só Go, e o build do plugin sai do caminho crítico dos PRs do servidor
- Mudança que cruza os dois passa a exigir dois PRs coordenados, com a API REST como
  contrato entre eles (ADR-0026)
- O contrato deixa de ser verificado por um CI único: quebrar a API aqui só aparece no CI
  do plugin
- A ADR-0004 (plugin como interface) e a ADR-0011 (source control view) saem daqui e viram
  a ADR-0002 e a ADR-0003 do repositório do plugin, junto com a visão C4 e os testes de
  aceitação. A numeração daqui fica com dois buracos
- A visão C4 do sistema, que cobria também os componentes do servidor, passa a ser mantida
  no repositório do plugin
