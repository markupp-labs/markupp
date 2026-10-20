# ADR-0041: Configuração sem fricção no self-host

## Status

Aceita

## Contexto

Subir o markupp exigia um segredo gerado à mão, três variáveis do OAuth e o mesmo segredo
repetido no servidor MCP. Projetos de home lab como Gitea, Nextcloud e Vaultwarden sobem com
uma URL pública e geram os próprios segredos na primeira subida, guardando-os no estado da
aplicação; pedir segredo à mão é a reclamação mais comum nos que não fazem isso

## Decisão

O servidor Go gera na primeira subida o segredo do JWT e o segredo do servidor MCP como
resource e os guarda no banco, que todas as réplicas leem. A variável de ambiente de cada um
continua valendo como override, que é como o cluster opera. `MARKUPP_PUBLIC_URL` passa a ter
padrão (`http://localhost` na porta do servidor), o OAuth vem ligado com o registro dinâmico
desligado, e o servidor MCP fica na mesma origem, em `/mcp`, encaminhado pelo Go ao container
dele. Sem `MARKUPP_OAUTH_RESOURCES`, o único resource é esse `/mcp`. O Go escreve o segredo do
MCP num arquivo de volume compartilhado, e o servidor MCP descobre o issuer pela metadata do Go

## Alternativas consideradas

- Segredos em arquivos de um volume de dados, como o Vaultwarden: um volume só atende uma
  réplica do Go
- Servidor MCP como processo filho na imagem do Go: junta dois ciclos de vida e duas
  toolchains na mesma imagem
- Script que gera o `.env` antes da primeira subida: continua sendo um passo que o usuário
  precisa conhecer
- Servidor MCP em porta própria: uma porta e uma entrada a mais no proxy reverso

## Consequências

- `docker compose up` sobe tudo sem `.env`; mudar a URL pública é a única configuração comum
- O segredo do JWT no banco é lido por quem tem acesso ao banco, o mesmo alcance de quem já lê
  as notas
- A rota `/mcp` dispensa o prazo de escrita do servidor, porque o Streamable HTTP mantém
  streams abertos
- Quem usa o registro dinâmico precisa ligá-lo; Claude, Cursor e VS Code usam CIMD
- O cluster continua com o servidor MCP em subdomínio próprio (ADR-0038), definindo as
  variáveis por Secret
