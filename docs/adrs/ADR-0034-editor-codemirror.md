# ADR-0034: CodeMirror 6 como editor de nota do painel

## Status

Aceita

## Contexto

O servidor nunca altera o conteúdo de uma nota, e as mesmas notas são editadas no Obsidian e
por agentes. O editor do painel precisa devolver o markdown exatamente como recebeu, sem
trocar marcador de lista, escape, linha em branco ou wikilink. Editores que mantêm um modelo
de documento próprio e serializam markdown a partir dele normalizam o texto ao salvar

## Decisão

O painel edita a nota com CodeMirror 6, cujo documento é o próprio texto, com pré-visualização
ao vivo feita por extensão do projeto. Os pacotes são vendorizados pelo importmap

## Alternativas consideradas

- TipTap, Milkdown, Lexical e BlockNote: edição rica pronta, e reformatam o markdown ao
  serializar do modelo de documento
- OverType: um arquivo pequeno com texto exato, e projeto jovem de um mantenedor, com
  alinhamento frágil no celular e sem ocultar a sintaxe
- Textarea com pré-visualização no servidor: zero JavaScript e texto exato, sem a edição densa
  que o painel quer

## Consequências

- A pré-visualização ao vivo e o destaque de wikilink são código nosso, mantido no painel
- O painel carrega JavaScript de terceiro, vendorizado, então o self-host não depende de CDN
- Ler e editar são a mesma superfície, e o servidor não renderiza markdown
