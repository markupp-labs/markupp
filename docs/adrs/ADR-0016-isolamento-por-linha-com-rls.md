# ADR-0016: Isolamento de cliente por linha, com row-level security

## Status

Aceita

## Contexto

O Enterprise é uma instância única compartilhada, com divisão lógica por cliente, para
aproveitar economia de escala e não deixar recurso ocioso. É preciso garantir que a consulta
de um cliente nunca alcance dado de outro, inclusive em travessia de grafo, onde até a
contagem de backlinks entrega a existência de nota alheia

## Decisão

Cada tabela recebe `tenant_id`, com row-level security do PostgreSQL e política que restringe
as linhas ao tenant da sessão. A aplicação define o tenant uma vez por transação e não
repete filtro em query nenhuma. O self-host é o mesmo desenho com um único tenant

## Alternativas consideradas

- Schema por cliente: migration multiplicada pelo número de clientes, milhares de relações no
  catálogo do Postgres, e a disciplina de acertar `search_path` em cada conexão troca um
  problema por outro
- Banco por cliente: melhor isolamento e pior uso de recurso, contrário à economia de escala
  pretendida
- Filtrar por `tenant_id` só na aplicação, sem RLS: uma query sem `WHERE` vaza tudo, e a
  garantia passa a depender de ninguém esquecer

## Consequências

- Vazamento entre clientes passa a ser impedido pelo banco, não pela disciplina de quem
  escreve query
- A aplicação precisa conectar com papel que não seja dono da tabela, e a tabela precisa de
  `FORCE ROW LEVEL SECURITY`, porque dono e superusuário ignoram política por padrão
- A política de isolamento precisa de teste próprio: uma política errada vaza tudo de uma vez
- Self-host e Enterprise compartilham o mesmo caminho de código, sem modo alternativo
- Cliente grande e cliente pequeno dividem o mesmo índice, e vizinho ruidoso só se resolve
  depois, com particionamento por `tenant_id`
