# ADR-0058: Self-host em porta única e mesma origem

## Status

Aceita

## Contexto

O self-host não tinha caminho de instalação próprio: o compose da raiz era o de
desenvolvimento, o painel e a API publicavam portas diferentes (8080 e 3000, com 3999 como
padrão da URL do painel no Go) e o self-host precisava de duas URLs, a pública e a do painel,
repetidas em mais de um serviço. O painel não podia dividir a origem da API porque o alias da
API sem `/v1` ocupava a raiz com os mesmos caminhos do painel

## Decisão

O `docker-compose.yaml` da raiz é o compose de self-host, com as imagens publicadas, e o
desenvolvimento usa `compose.dev.yaml`. O servidor Go escuta na porta 6275, o mesmo número
dentro e fora do container, e é a única porta publicada; `MARKUPP_PORT` troca só o lado do
host. O Go encaminha ao painel todo caminho que não atende, como já faz com `/mcp`, quando
`MARKUPP_PANEL_UPSTREAM_URL` aponta o container do painel. Sem `MARKUPP_PANEL_URL`, a URL do
painel é a própria `MARKUPP_PUBLIC_URL`. O alias da API sem `/v1` sai, e o consentimento do
painel passa a `/oauth/requests/<id>`, porque `/oauth/authorize` é do Go

## Alternativas consideradas

- Painel e API em portas próprias: duas entradas no proxy reverso e duas URLs para configurar
- Alias sem `/v1` convivendo com o painel na raiz, separado por cabeçalho: heurística no
  roteador para um contrato já obsoleto
- Proxy reverso próprio no compose, como Caddy: um serviço a mais para fazer o que o Go já faz
  com o MCP

## Consequências

- Uma porta, uma origem e uma URL: mudar a URL pública volta a ser a única configuração comum
- Cliente que chama a API sem `/v1`, como o plugin do Obsidian anterior à migração, recebe 404
- O cluster mantém o painel em host próprio, sem `MARKUPP_PANEL_UPSTREAM_URL` e com
  `MARKUPP_PANEL_URL` definida
- 6275 está numa faixa sem registro na IANA (6270-6299) e não é padrão de outro projeto
  conhecido
