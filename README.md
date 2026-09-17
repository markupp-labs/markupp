# Markupp

Base de conhecimento em markdown que se organiza sozinha. Você escreve e salva, e o servidor
liga uma nota à outra e agrupa por tema, sem nunca mexer no conteúdo. Nenhuma árvore de pastas
para manter na mão.

A busca é otimizada para encontrar por sentido, e não só pela palavra digitada. Procurar por
"como cobramos por uso" chega na nota que fala em tarifação por consumo.

Quem faz o trabalho é o servidor. Os clientes são o painel web, plugins de editor e agentes de
IA, e toda operação disponível a uma pessoa está disponível a um agente.

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
