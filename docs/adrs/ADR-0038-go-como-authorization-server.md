# ADR-0038: Servidor Go como authorization server OAuth

## Status

Aceita

## Contexto

O servidor MCP remoto precisa autenticar agentes como Claude, Cursor e ChatGPT, e a
especificação do MCP pede OAuth 2.1 para isso. Hoje a API só emite JWT curto com refresh
rotativo a partir de email e senha. A especificação também proíbe o servidor MCP de repassar à
API o token que recebeu do agente: o token precisa ter o servidor MCP como destinatário

## Decisão

O servidor Go vira authorization server OAuth 2.1, com authorization code e PKCE. Clientes se
registram por Client ID Metadata Document, com Dynamic Client Registration como alternativa
para quem só suporta esse. A Autorização carrega escopos (ler notas, escrever notas,
administrar) que a conta vê ao consentir. O token que o agente recebe é opaco, guardado como
hash, e tem o servidor MCP como audiência. O servidor MCP o troca por um token curto da API pela
troca de token da RFC 8693, autenticando-se com um segredo próprio, e o token da API leva os
escopos que a API aplica por rota. O servidor MCP não valida o token por conta própria: se a
troca falhar, responde 401. Na versão stdio, um
comando de login faz o mesmo fluxo no navegador com redirect para loopback e guarda o refresh
token na máquina

## Alternativas consideradas

- Servidor MCP como authorization server, autenticando na API com login próprio: nenhuma
  mudança no Go, e o servidor MCP passaria a guardar sessões e credenciais
- Provedor de identidade externo no cluster, como Keycloak ou Zitadel: mais uma peça para
  operar, pesada para o self-host
- Token válido para MCP e API ao mesmo tempo: seria repasse disfarçado, e um token vazado
  alcançaria a API inteira
- Credencial de serviço do servidor MCP agindo em nome da conta: tornaria o servidor MCP
  cliente privilegiado, contra o ADR-0026
- Token pessoal de longa duração: resolveria o stdio, e os conectores remotos dos clientes de
  IA continuariam exigindo OAuth

## Consequências

- A Autorização vira estado do servidor Go, que a conta concede e pode revogar
- Um token do agente vazado não chega à API sem o segredo do servidor MCP
- Tokens de login continuam sem escopo e com o acesso inteiro da conta
- Outros clientes, como o plugin, passam a poder usar o mesmo fluxo em vez de email e senha
- Cada requisição ao servidor MCP sem token trocado em cache custa uma chamada de troca ao Go
- O DCR abre registro anônimo de clientes, que precisa de limite de taxa e expiração
- O JWT continua HMAC com segredo do Go, porque nenhum outro serviço valida token
