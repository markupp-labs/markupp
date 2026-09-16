# Markupp

Servidor de notas em markdown que organiza a base sozinho. O usuário escreve e salva, e o
servidor indexa, liga e agrupa sem alterar o conteúdo. Toda interação passa por uma API REST
versionada, e painel web, plugin de editor e agente de IA são clientes pares dela.

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
