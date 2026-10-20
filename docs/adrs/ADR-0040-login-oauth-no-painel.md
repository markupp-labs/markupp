# ADR-0040: Login e consentimento OAuth no painel

## Status

Aceita

## Contexto

O authorization server do ADR-0039 precisa de telas para a conta entrar e consentir a
Autorização de um cliente. O servidor Go não renderiza HTML hoje, e o painel Rails do ADR-0031
é a interface web do sistema, mas vive numa branch de longa duração sem previsão de merge

## Decisão

As telas de login e consentimento ficam no painel. O `/oauth/authorize` do Go redireciona ao
painel com o identificador do pedido. O painel autentica a conta pela API, mostra o pedido e
chama um endpoint do Go que o aprova ou recusa. O Go emite o código e redireciona ao cliente.
O painel continua sem alcançar o banco

## Alternativas consideradas

- HTML mínimo renderizado pelo próprio Go: tira a dependência do painel e funciona no
  self-host sem ele, ao custo de uma segunda interface web para manter
- Sessão por cookie compartilhada entre painel e Go: exigiria criar sessão por cookie no Go e
  acoplaria os dois pelo domínio

## Consequências

- O OAuth só funciona de ponta a ponta depois do merge do painel, e o servidor MCP, nos dois
  transportes, herda esse prazo
- O self-host precisa subir o painel para que qualquer cliente OAuth funcione
- O endpoint de aprovação é parte do contrato da API e só aceita a conta autenticada dona do
  pedido
