# ADR-0034: Isolamento por linha com papel da aplicação por transação

## Status

Aceita

## Contexto

O ADR-0016 decidiu isolar cliente por linha com row-level security e pediu que a aplicação
conectasse com papel que não fosse dono das tabelas. Na prática, a aplicação conecta com o
usuário da imagem do PostgreSQL, que é superusuário e ignora qualquer política. E parte do
trabalho acontece antes de existir tenant: login, refresh e aceite de convite só descobrem o
tenant depois de ler a conta

## Decisão

Cada tabela com dado de cliente recebe `tenant_id` e política de RLS. A conexão continua com o
usuário da instalação, e toda transação de dados troca, com `SET LOCAL`, para o papel
`markupp_app`, sem login, sem superusuário e sem `BYPASSRLS`, e entrega ao banco o tenant, o
usuário e o papel dele, tirados do JWT. As políticas recortam por tenant, por pertencimento ao
cofre (ADR-0024) e, em usuários e convites, pelo papel de administrador. Nenhuma query repete o
filtro. Autenticação roda no papel da conexão, porque precede o tenant

## Alternativas consideradas

- Schema por cliente: migration multiplicada pelo número de clientes, milhares de relações no
  catálogo do Postgres, e a disciplina de acertar `search_path` em cada conexão
- Banco por cliente: melhor isolamento e pior uso de recurso, contrário à economia de escala
- Filtrar por `tenant_id` só na aplicação, sem RLS: uma query sem `WHERE` vaza tudo
- Conectar a aplicação com papel de login sem privilégio e migrar por outro DSN: soma um DSN e
  um segredo por ambiente, e login, refresh e aceite de convite precisariam de funções
  `SECURITY DEFINER` para atravessar o RLS

## Consequências

- Vazamento entre clientes e entre cofres passa a ser impedido pelo banco
- Instalação e deploy não mudam: o papel nasce na migração
- Um repositório que use o pool fora da transação do ator roda como superusuário e enxerga todos
  os tenants. Um teste lê o pacote de armazenamento e falha quando isso acontece
- `FORCE ROW LEVEL SECURITY` fica dispensado, porque `markupp_app` não é dono das tabelas
- A política de isolamento tem teste próprio, que roda no papel da aplicação sem `WHERE`
- Self-host e Enterprise compartilham o mesmo caminho de código
- Cliente grande e cliente pequeno dividem o mesmo índice, e vizinho ruidoso só se resolve
  depois, com particionamento por `tenant_id`
