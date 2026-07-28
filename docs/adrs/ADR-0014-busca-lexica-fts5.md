# ADR-0014: Busca léxica com FTS5 antes do grafo de ligações

## Status

Proposta

## Contexto

O plano de recuperação (ADR-0012) admite várias estruturas derivadas, e duas disputam a
primeira implementação: um índice léxico de texto completo e o grafo de ligações extraídas
do markdown. A busca atual é `content LIKE` ordenada por `updated_at`, sem relevância e sem
trecho no resultado, então o agente recebe uma lista de caminhos e precisa buscar cada nota
para descobrir se serve

## Decisão

Implementar a busca léxica com FTS5, nativo do SQLite, antes do grafo de ligações. FTS5
entrega ranking bm25 e `snippet()` sem dependência nova nem serviço novo, e ataca o custo em
token que o agente paga hoje. O grafo de ligações vem depois

## Alternativas consideradas

- Grafo de ligações primeiro: é o mecanismo que o ADR-0013 exige para organização derivada,
  mas depende da densidade de wikilink do acervo, que ainda não foi medida, e não reduz o
  custo em token da busca atual
- Motor de busca externo (Elasticsearch, Meilisearch): contraria o ADR-0003 e exige
  infraestrutura que o usuário não precisa hoje
- As duas estruturas em paralelo: dobra a superfície em mudança na camada de recuperação sem
  que uma dependa da outra

## Consequências

- Ganho imediato de relevância e de economia de token, sem dependência nova
- FTS5 sozinho não organiza nada: entrega busca melhor, não a tese do ADR-0013
- Medir a densidade de ligação do acervo segue pendente e continua pré-requisito para
  dimensionar o grafo
- Se o plano de recuperação morar em arquivo separado do registro, o índice guarda a própria
  cópia do conteúdo em vez de referenciar a tabela de notas, e o custo em disco dobra
- Ordem revisável: densidade de ligação alta o bastante inverte a inclinação
