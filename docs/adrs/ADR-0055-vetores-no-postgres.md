# ADR-0055: Vetores no Postgres com pgvector

## Status

Proposta

## Contexto

A ligação por sentido (ADR-0053) guarda um vetor por nota e acha, para cada nota, as 5 mais
próximas no mesmo cofre. O vetor muda quando a nota muda, e o kNN é refeito no segundo estágio,
junto com o resto do grafo do cofre

## Decisão

Os vetores ficam no próprio Postgres, com o pgvector, numa tabela com a dimensão do modelo em
uso e um índice HNSW de cosseno. O segundo estágio faz o kNN mútuo em SQL, com a busca iterativa
estrita para o filtro do cofre não cortar vizinhos. Trocar de modelo apaga os vetores, refaz a
coluna e o índice e reenfileira todas as notas

O Postgres de todas as instalações passa a ser a imagem oficial do pgvector, em Debian. O banco
externo precisa ter o pgvector instalado

## Alternativas consideradas

- Banco vetorial à parte, como o Qdrant: mais um serviço para quem faz self-host subir e manter
- Vetores na memória do indexador: refaz tudo a cada subida e não cabe num cofre grande
- Imagem própria em Alpine com o pgvector compilado: mantém a ordenação do texto do banco que
  já existe, mas o projeto passa a construir e publicar mais uma imagem
- Índice IVFFlat: precisa de dados para treinar e perde recall numa tabela que cresce

## Consequências

- Quem faz self-host continua com um banco só, numa imagem mantida pelo pgvector
- Trocar musl por glibc muda a ordenação do texto: a migração refaz os índices das tabelas
  uma vez, e a atualização trava as tabelas enquanto isso
- O banco externo sem pgvector não migra até alguém instalar a extensão
- Modelo de mais de 2000 dimensões fica sem índice, e o kNN lê o cofre inteiro
