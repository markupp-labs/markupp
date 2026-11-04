# ADR-0020: Autenticação com provedores configuráveis

## Status

Aceita

## Contexto

O acesso precisa ser controlado por autenticação e autorização, inclusive por provedor externo
de identidade. As instalações têm restrições opostas: o self-host não pode depender de provedor
de terceiro para o primeiro login, e uma instalação com vários tenants não deveria armazenar
senha nem operar fluxo de recuperação. O Langfuse, cujo modelo de distribuição é o mesmo do
markupp, resolve isso com lista de provedores habilitada por configuração no mesmo binário

## Decisão

Autenticação com provedores habilitados por configuração: credencial local, com usuário e
senha, e qualquer número de provedores OIDC ou OAuth, como Google e GitHub. Uma instalação com
vários tenants habilita provedor externo, e o self-host escolhe, inclusive ficar só na
credencial local. Mesmo desenho do ADR-0019: uma interface, implementações escolhidas por
configuração

## Alternativas consideradas

- Só credencial local, no modelo do Jellyfin: não atende autenticação por provedor externo, e
  obriga a instalação com vários tenants a guardar senha e operar recuperação de acesso
- Só OIDC: obriga o self-host a depender de provedor de terceiro para conseguir entrar
- OIDC com vários tenants e credencial local no self-host, fixos: divide o código de
  autenticação por instalação, e tira da organização que hospeda por conta própria a opção de
  plugar o provedor de identidade que ela já usa

## Consequências

- Nenhuma divergência de código entre instalações: a diferença entre elas é qual provedor está
  ligado
- Organização que hospeda por conta própria pode usar o próprio provedor de identidade
- Autenticação sai do escopo excluído pelo ADR-0005
- A conta precisa de chave estável própria, com o identificador do provedor guardado à parte,
  senão trocar de provedor ou de e-mail perde o vínculo com o histórico do usuário
- Primeiro acesso precisa de regra de quem se torna administrador, e auto-registro fica
  desligado por padrão

## Emenda

Em novembro de 2026, a edição operada pela equipe passou a se chamar Cloud. A decisão não
mudou.
