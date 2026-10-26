# ADR-0048: Cofre público lido sem conta

## Status

Proposta

## Contexto

Quem usa o markupp quer abrir um cofre para leitura de qualquer pessoa, e a documentação do
próprio markupp deve ser o primeiro cofre assim, servindo de demonstração ao vivo. Hoje toda rota de nota
exige conta, e o RLS do ADR-0034 recorta pelo tenant e pelo usuário entregues pela transação do
ator. Não existe caminho de leitura para quem não tem conta

## Decisão

A unidade publicada é o cofre, não o tenant. O cofre já é a unidade de acesso e o namespace
(ADR-0024), e as políticas das notas já recortam por cofre. Publicar o tenant exporia de uma vez
cofres privados que convivem com o público.

A publicação fica numa tabela própria, `vault_publications`, com o cofre, o tenant, um slug
global único e quem publicou. Só owner e admin do tenant publicam e despublicam, e cada ação
gera evento de auditoria.

A leitura pública roda num papel novo, `markupp_reader`, sem login, sem superusuário e sem
`BYPASSRLS`, ativado por `SET LOCAL` numa transação própria, paralela à do ator. O papel só
recebe `SELECT`, e só nas tabelas de nota e do plano de recuperação, com política que limita
aos cofres publicados. Contas, tenants, vínculos, membros de cofre, convites, auditoria e a
caixa de saída de emails ficam sem grant.

A API ganha rotas `/v1/public/vaults/{slug}/...` sem autenticação, com CORS aberto para `GET`,
limite por IP e `ETag`. Cofre inexistente e cofre não publicado respondem igual, 404, para não
revelar que o cofre existe. O painel serve a leitura em `/p/{slug}`, fora do login

## Alternativas consideradas

- Tenant público: expõe todos os cofres do tenant ou exige exceção por cofre, e não significa
  nada no self-host, que tem um tenant só
- Coluna `public` em `vaults`: mexe nas políticas de uma tabela central e mistura publicação,
  que é evento auditável, com o estado do cofre
- Exportar o cofre como site estático: a publicação deixa de refletir a edição na hora, e o
  site perde a busca e o grafo
- Ler com o papel da conexão e filtrar na aplicação: uma query sem `WHERE` vaza tudo, o mesmo
  motivo que levou ao ADR-0016

## Consequências

- Leitura anônima é impedida pelo banco de alcançar qualquer cofre não publicado, e o papel não
  escreve nada
- A guarda de testes que exige a transação do ator passa a aceitar a transação pública como
  segunda entrada
- As rotas públicas viram superfície sem autenticação, e o limite por IP é a defesa contra
  raspagem e abuso de busca
- O slug é global entre tenants, então o primeiro a reservar fica com ele
