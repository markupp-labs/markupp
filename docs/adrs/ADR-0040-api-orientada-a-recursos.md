# ADR-0040: API orientada a recursos

## Status

Aceita

## Contexto

O ADR-0007 fez da API REST a única interface, e o ADR-0026 a tornou contrato público versionado,
mas nenhum dos dois diz como as rotas são desenhadas. A organização derivada acrescenta ligações,
menções, sugestões, comunidades e grafo, e cada uma pedia uma rota própria por visão (órfãs, hubs,
vizinhança, não resolvidas), o que espalha o mesmo recurso por várias URLs e obriga o cliente a
decorar cada uma

## Decisão

A API é orientada a recursos, com interface uniforme:

- Todo recurso é coleção ou item. A coleção vive sob o recurso que a contém (as notas do cofre em
  /vaults/{id}/notes), e o item com identificador global é endereçado direto (/notes/{id})
- Relação entre recursos é sub-coleção do recurso de origem, uma por sentido: ligações de saída
  em /notes/{id}/links e de entrada em /notes/{id}/backlinks
- A coleção devolve a representação enxuta do item, sem o conteúdo, e o item completo vem da rota
  dele: listar as notas do cofre não lê nem transfere o texto de cada uma
- Visão sobre uma coleção é consulta, não rota: filtro, ordenação e paginação por parâmetros
  (?orphan=true, ?sort=pagerank&direction=desc), com os valores derivados como campos do recurso
- Busca é recurso próprio em /search, porque a representação do resultado (relevância e trecho)
  difere da representação do item
- O grafo para desenhar é representação própria do cofre (/vaults/{id}/graph) e da nota
  (/notes/{id}/graph): nós e ligações como pares de índices nos nós, sem paginação, recortado
  por parâmetros, comprimido quando o cliente aceita e validado pela versão do grafo do cofre
- Os nomes das rotas são os termos do glossário do domínio (CONTEXT.md)
- A superfície nova nasce sob /v1, e as rotas sem prefixo seguem servidas e marcadas obsoletas
  durante a transição do ADR-0026

## Alternativas consideradas

- Uma rota por visão: simples de escrever, e cada visão nova aumenta a superfície que o cliente
  precisa conhecer
- Coleção única de ligações filtrada por origem e destino: um recurso só, e perde a leitura direta
  de quem liga para uma nota
- Busca como filtro da coleção de notas: uma rota a menos, e a mesma URL passaria a devolver duas
  representações conforme o parâmetro

## Consequências

- Visão nova sobre um recurso existente entra como parâmetro, sem rota nova
- A nota carrega campos derivados (grau, PageRank, comunidade) que só existem depois do indexador
- Plugin e painel migram para /v1 no ritmo deles, porque as rotas antigas continuam respondendo
- Manter as duas superfícies durante a transição custa código e teste, como o ADR-0026 já previa

## Emenda

Durante a transição, a coleção antiga do ADR-0026 (/v1/notes, /v1/notes/search e POST /v1/notes),
servida sob /v1 pelo prefixo de versão e usada pelo servidor MCP, convive sob /v1 com os recursos
deste ADR. /v1/notes/{id} usa a representação nova, superset da antiga. As notas do cofre, a
busca, as ligações e o grafo aceitam também o token OAuth com o escopo de leitura de notas
(notas:leitura, ADR-0046)
