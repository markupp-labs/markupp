# ADR-0059: Cadastro aberto por método, no banco, com a configuração de quem hospeda vencendo

## Status

Proposta

## Contexto

O auto-registro só se liga e desliga por inteiro, em `MARKUPP_AUTH_REGISTRATION_ENABLED`, e vale
igual para email e senha e para cada provedor externo (ADR-0052, ADR-0054). Quem já tem onde as
pessoas se identificam quer que contas novas nasçam só por esse provedor, sem fechar o login por
senha de quem já existe e sem editar variável e reiniciar

## Decisão

Cada método de cadastro, email e senha (`local`) e cada provedor configurado, tem seu interruptor.
O administrador da instância o muda pela API, em `/v1/instance/registration-methods`, e a escolha
fica no banco; método sem escolha está aberto. Com `MARKUPP_AUTH_LOCAL_REGISTRATION` ou
`MARKUPP_LOGIN_<ID>_REGISTRATION` definida, vale a variável e o interruptor daquele método fica
travado, como no provedor de embedding (ADR-0057)

`MARKUPP_AUTH_REGISTRATION_ENABLED` continua como interruptor geral: desligada, nenhum método
cadastra, todos aparecem travados para o administrador, e a primeira conta da instalação continua
nascendo (ADR-0052). Fechar o cadastro de um
método não fecha o login por ele; quem desliga o login por senha segue usando
`MARKUPP_AUTH_LOCAL_ENABLED`. O cadastro fechado responde `registration_disabled`, e o painel
esconde o que está fechado lendo `/v1/login-providers` e `/v1/accounts/policy`

## Alternativas consideradas

- Só variáveis por método, como o Gitea: trocar exige acesso ao servidor e reinício
- O banco vencendo as variáveis, como o Grafana e o Vaultwarden: quem hospeda perde o controle e
  a variável passa a mentir
- Um interruptor para senha e outro para todos os provedores: não deixa abrir só um provedor
- O interruptor fechando também o login: quem já tem conta por aquele método fica de fora

## Consequências

- Quem faz self-host fecha o cadastro por senha e deixa só o do provedor pelo painel
- O convite continua criando conta com o cadastro fechado
- Provedor removido da configuração deixa a escolha guardada sem efeito, e ela volta a valer se o
  mesmo id voltar
