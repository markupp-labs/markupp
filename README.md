# Markupp

Base de conhecimento em markdown que se organiza sozinha. Você escreve e salva, e o servidor
liga uma nota à outra e agrupa por tema, sem nunca mexer no conteúdo. Nenhuma árvore de pastas
para manter na mão.

Pessoas e agentes de IA trabalham sobre a mesma base, com o mesmo conjunto de operações.

Descrição completa em [resumo do projeto](docs/resumo.md).

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
