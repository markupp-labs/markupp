# ADR-0054: Login por provedor externo

## Status

Proposta

## Contexto

Hoje a conta só entra por email e senha. Quem instala o markupp muitas vezes já tem onde as
pessoas se identificam, seja um IdP próprio (Keycloak, Authentik) ou contas do Google e do
GitHub, e quer evitar mais uma senha. A tabela de identidades já separa a conta da forma de
entrar, com unicidade por provedor e identificador no provedor

## Decisão

A instalação pode configurar provedores externos de login: qualquer provedor OpenID Connect e o
GitHub, que não fala OIDC e ganha um adaptador próprio. Sem configuração, nada muda e o botão não
aparece. O servidor Go troca o código com o provedor e emite a mesma sessão do login local; o
painel só redireciona e mostra o botão.

A identidade externa só entra em conta que já a tem vinculada. O vínculo a uma conta existente
nasce de uma sessão ativa, nas configurações da conta, nunca pelo email que o provedor informa.
Primeiro acesso sem vínculo é cadastro explícito e segue o auto-registro, como no ADR-0052

## Alternativas consideradas

- Só OIDC genérico: menos código, deixa o GitHub de fora
- Vincular automaticamente por email verificado: mais cômodo, e quem controla o email no provedor
  entra na conta local
- Callback no painel com biblioteca pronta: o painel passa a guardar segredo de provedor e a API
  precisa confiar na identidade que ele afirma

## Consequências

- Clientes OAuth, inclusive o MCP, herdam o login externo pela tela do painel (ADR-0047)
- Com provedor configurado, a instalação pode desligar a credencial local
- Conta só com identidade externa não tem senha para recuperar; perder o acesso ao provedor exige
  outra identidade vinculada ou ação de admin
- Cada provedor configurado é mais um segredo da instalação
