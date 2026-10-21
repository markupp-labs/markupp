# ADR-0042: Login do servidor MCP em stdio

## Status

Aceita

## Contexto

O ADR-0046 previa que o servidor MCP em stdio fizesse o mesmo fluxo do HTTP, trocando o token
do agente pelo da API. A troca exige o segredo do resource, e um pacote npm instalado na máquina
da pessoa não tem como guardar segredo. O registro dinâmico, que um CLI usaria para se
registrar, vem desligado (ADR-0041), e o Client ID Metadata Document exigiria que o servidor
buscasse um documento na internet

## Decisão

O servidor Go conhece de fábrica um cliente público, `markupp-mcp`, com redirect só em loopback
e PKCE, como os CLIs do GitHub e do Google Cloud. O comando `markupp-mcp login` abre o
navegador e pede como resource o próprio issuer, que já é o identificador da API na troca de
token. Para esse resource, o Go entrega como access token o JWT curto da API, sem gravá-lo, e
um refresh opaco e rotativo como os demais. O processo stdio guarda o refresh num arquivo
modo 0600 em `~/.config/markupp` e renova o JWT quando ele vence, com trava de arquivo para
dois processos não reaproveitarem o mesmo refresh

## Alternativas consideradas

- Resource público, com troca sem segredo: enfraquece a autenticação da troca para todos os
  resources
- Client ID Metadata Document publicado pelo projeto: o servidor self-host precisaria de
  internet para buscar o documento
- Registro dinâmico: obrigaria quem opera a ligá-lo
- Chaveiro do sistema: dependência nativa no `npx` e falha onde não há Secret Service

## Consequências

- O JWT entregue ao stdio vale até expirar mesmo depois da revogação, como o da troca, com a
  mesma validade curta
- O refresh no disco alcança a API inteira dentro dos escopos concedidos; revogar a
  Autorização no painel o derruba
- Revogar um JWT pelo `/oauth/revoke` não tem efeito, porque ele não é gravado
- Dois processos stdio na mesma máquina dividem uma Autorização só
