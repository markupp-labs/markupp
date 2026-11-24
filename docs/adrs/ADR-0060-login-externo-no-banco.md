# ADR-0060: Provedores de login externo no banco, com a configuração de quem hospeda vencendo

## Status

Proposta

## Contexto

Os provedores de login externo (ADR-0054) só vinham das variáveis `MARKUPP_LOGIN_*`, lidas
quando a API sobe. Quem hospeda no Kubernetes precisava mexer no deploy para ligar o Google ou o
GitHub. O vínculo de cada conta guarda só o id do provedor e o id da pessoa nele

## Decisão

O administrador da instância mantém a lista de provedores pela API, pelo painel e pelo MCP: id,
nome, issuer (vazio só no id `github`), client id, client secret e se o provedor está ligado. O
segredo fica cifrado como a chave do embedding (ADR-0057), e nenhuma saída o devolve. A API relê
a lista quando ela muda, sem reiniciar, e só o provedor alterado refaz a descoberta OIDC

Com `MARKUPP_LOGIN_PROVIDERS` definida, vale só a configuração de quem hospeda, e a lista do
banco fica travada

O id não muda depois de criado. O issuer também não muda enquanto alguma conta estiver vinculada
ao provedor, porque o mesmo id de pessoa em outro issuer pode ser outra pessoa. Apagar o provedor
apaga junto os vínculos das contas com ele, e o painel avisa antes quantas contas perdem esse
jeito de entrar: um provedor novo com o mesmo id começa sem vínculo

## Alternativas consideradas

- Só variáveis, como antes: ligar um provedor pede acesso ao deploy e reinício
- O banco somando com as variáveis: duas fontes para a mesma lista, com regra de colisão de id
- Recusar apagar enquanto houver vínculo: o administrador fica sem como tirar um provedor comprometido
- Manter os vínculos órfãos: um provedor novo com o mesmo id religa contas antigas

## Consequências

- Quem faz self-host liga o login pelo Google e pelo GitHub pelo painel
- Quem só entrava pelo provedor apagado perde o acesso, e volta pela troca de senha por email, se
  a conta tiver senha, ou por um convite
- O servidor busca a descoberta no issuer que o administrador escolheu, com a regra https do
  ADR-0054
- Cada réplica da API vê a mudança em alguns segundos, quando relê a lista
