# ADR-0013: Organização derivada, sem alterar notas

## Status

Aceita

## Contexto

O markupp quer manter a base organizada e barata de recuperar para agentes de IA e para
clientes humanos. Organizar pode significar reagrupar, fundir e renomear notas. Mas o
markupp também devolve ao cliente o que o cliente enviou, e mover ou reescrever nota
quebra essa devolução

## Decisão

A organização é derivada. O servidor calcula ligações, agrupamentos e índices em paralelo
e nunca altera o conteúdo nem o path de uma nota. Os clientes podem exibir a visão
derivada no lugar da estrutura de pastas

## Alternativas consideradas

- Organização sugerida, aplicada mediante aprovação: exige fila de propostas e interface
  própria em cada cliente
- Organização autônoma pelo servidor: a nota volta diferente do que foi enviada e o vault
  do usuário muda sem ação dele

## Consequências

- O cliente recebe de volta a estrutura que enviou
- Trocar o algoritmo de organização não arrisca dado do usuário
- O ganho de organização depende de os clientes exibirem a visão derivada
- Nota mal organizada continua mal organizada no disco do usuário
- Adotar organização sugerida ou autônoma no futuro exige substituir este ADR
