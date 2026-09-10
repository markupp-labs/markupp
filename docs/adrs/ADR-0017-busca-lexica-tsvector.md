# ADR-0017: Busca léxica com relevância e trecho no resultado

## Status

Aceita

## Contexto

A busca hoje é `content LIKE '%termo%'` ordenada por `updated_at`, e devolve `id`, `path` e
`updated_at` sem trecho nenhum. Relevância não participa: a nota editada mais recentemente que
contenha a substring vem primeiro, ainda que mencione o termo de passagem. O cliente recebe uma
lista de caminhos e precisa buscar cada nota para descobrir se serve, o que para um agente de
IA significa gastar contexto em nota irrelevante antes de saber que é irrelevante

Casamento por substring também não tem radicalização nem peso por termo: "organização" não
encontra "organizar", e termo que aparece em toda nota conta igual a termo raro

## Decisão

Busca léxica com ranking de relevância e trecho no resultado, usando `tsvector`, `ts_rank` e
`ts_headline` do PostgreSQL. É a primeira estrutura do plano de recuperação (ADR-0012), e vem
antes do grafo de ligações

## Alternativas consideradas

- Manter `content LIKE` (status quo): sem relevância, sem trecho e sem radicalização, é a causa
  do problema
- FTS5 do SQLite, proposto no ADR-0014: traz BM25, que é ranking melhor, mas o ADR-0015 tirou o
  SQLite do projeto
- ParadeDB com `pg_search`: entrega BM25 sobre Postgres, e o custo de extensão é baixo porque
  pgvector já obriga a sair da imagem oficial. O impedimento é o Enterprise: `pg_search` não
  está na lista de extensões suportadas pelo RDS, enquanto pgvector está, então adotá-lo
  obrigaria a operar Postgres por conta própria ou a manter busca diferente por edição
- Motor de busca externo (Elasticsearch, Meilisearch): mais um serviço para operar nas duas
  edições, para um ganho que `tsvector` já entrega
- Ir direto para busca semântica: depende de provedor de embedding configurado (ADR-0019), tem
  custo por nota, e erra justamente em termo exato, nome próprio e identificador. Não substitui
  a léxica nem é pré-requisito para a melhoria imediata

## Consequências

- Substitui o ADR-0014
- O resultado passa a trazer trecho, então o cliente julga relevância sem abrir a nota. É o
  ganho direto em contexto consumido por agente
- Radicalização e stop words entram, então consulta e documento casam por radical em vez de por
  cadeia de caracteres
- A configuração de idioma do `tsvector` é por instalação: base com português e inglês
  misturados radicaliza pior, e a saída conservadora é a configuração `simple`, que desliga
  radicalização em troca de previsibilidade
- `ts_rank` não usa frequência inversa de documento, então termo comum e termo raro ainda pesam
  igual. Trocar por BM25 depois é mudança contida no plano de recuperação
- O índice GIN e a coluna `tsvector` vivem no plano de recuperação, mantidos pelo indexador
  (ADR-0018), e podem ser apagados e reconstruídos
