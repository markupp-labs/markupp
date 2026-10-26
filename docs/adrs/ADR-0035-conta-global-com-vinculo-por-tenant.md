# ADR-0035: Conta global com vínculo por tenant

## Status

Aceita

## Contexto

O Enterprise atende vários clientes na mesma instância, e a mesma pessoa pode trabalhar para
mais de um. Com a conta presa a um tenant, essa pessoa precisaria de um email por cliente, e
convidar um email que já tinha conta em outro cliente gerava convite impossível de aceitar. O
Langfuse, cujo modelo de distribuição é o mesmo do markupp (ADR-0020), resolve isso com
usuário da instância que participa de várias organizações, cada uma com seus projetos

## Decisão

A conta é da instância, com email único, e participa de vários tenants por vínculo com papel:
owner, admin, member ou viewer. O login não escolhe tenant. O JWT identifica só a pessoa, e
cada requisição diz em qual tenant age pelo cabeçalho `X-Markupp-Tenant`, dispensável para
quem tem um só. O servidor confere o vínculo no banco a cada requisição e usa o papel
registrado lá. Qualquer conta cria tenant e vira owner dele. O convite é um handshake: a conta
convidada vê os convites do próprio email e aceita ou recusa, e o token do convite serve só
para quem ainda não tem conta. Ninguém concede papel acima do próprio, e o tenant nunca fica
sem owner. O acesso às notas continua pelo cofre (ADR-0024): o papel no tenant governa a
administração, e o viewer só lê nos cofres de que é membro

## Alternativas consideradas

- Tenant informado no login, com identidade única por tenant: uma conta por cliente, e trocar
  de cliente exigiria novo login
- Subdomínio por tenant: os clientes configurariam só a URL, ao custo de DNS e certificado
  curinga
- Token emitido para um tenant e trocado por rota própria: o papel ficaria no JWT e
  envelheceria até o token expirar
- Papel no tenant dando acesso a todos os cofres, como os projetos do Langfuse: o cofre
  deixaria de ser a unidade de acesso que o ADR-0024 decidiu

## Consequências

- Os clientes listam os tenants da conta em `GET /me/tenants`, guardam o escolhido e mandam o
  cabeçalho
- Rebaixar ou remover alguém vale na requisição seguinte, sem esperar o token expirar, ao
  custo de uma consulta de vínculo por requisição
- Login, refresh e cadastro viram eventos da conta, auditados sem tenant
- Restringir quem cria tenant, como a lista de criadores do Langfuse, fica para quando o
  provisionamento externo pela API de admin pedir
- Sem verificação de email (ADR-0021), o token do convite é o que prova o email no cadastro. A
  conta de auto-registro fica fora do handshake até ter o email verificado, e com o
  auto-registro ligado qualquer pessoa ocupa um email alheio antes do dono
- O convite só vale enquanto quem convidou sustenta o papel dele no tenant, e pode ser revogado
- Quem entra num tenant ganha um cofre próprio lá. Quem sai perde os cofres em que era o único
  membro, com as notas; compartilhar cofre entre membros depende das rotas de cofre, ainda não
  implementadas
