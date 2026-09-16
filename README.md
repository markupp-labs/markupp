# Markupp

Servidor de notas em markdown que organiza a base sozinho. O usuário escreve e salva, e o
servidor indexa, liga uma nota à outra e agrupa por tema, sem alterar o conteúdo nem o caminho
do que foi escrito.

Toda interação passa por uma API REST versionada, e todo cliente é par dela. Nenhum alcança o
armazenamento direto.

## Rodando

```bash
docker compose up
```

Sobe em `localhost:8080`. Para usar a imagem publicada em vez de buildar do código, veja
[DEPLOY](docs/DEPLOY.md).

## Documentação

A API está descrita em [openapi.yaml](markupp/openapi.yaml). As decisões de arquitetura estão
registradas em [ADRs](docs/adrs/), e o resto vive em [docs](docs/).

## Licença

[AGPL v3](LICENSE).
