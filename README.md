[![CI](https://github.com/markupp-labs/markupp/actions/workflows/ci.yml/badge.svg)](https://github.com/markupp-labs/markupp/actions/workflows/ci.yml)
[![Licença](https://img.shields.io/github/license/markupp-labs/markupp)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/markupp-labs/markupp?filename=markupp%2Fgo.mod)](markupp/go.mod)

<p align="center">
  <img src="docs/assets/logo.png" alt="Markupp" width="160">
</p>

# Markupp

Markupp é uma base de conhecimento em markdown que se organiza sozinha. Quem faz o trabalho é
o servidor. Qualquer programa que leia ou escreva notas é cliente dele, nenhum privilegiado, e
o servidor não presume quais existem.

O usuário escreve e salva. O servidor indexa, liga uma nota à outra e agrupa por tema, sem
nunca alterar o conteúdo nem o caminho. O cliente mostra essa organização derivada no lugar
da árvore de pastas.

A busca é otimizada para encontrar por sentido. Ela acha a nota que trata do assunto mesmo
quando a palavra digitada não aparece nela, e devolve o trecho que casou com a pergunta.

O sistema nasce AI first. Toda operação disponível a uma pessoa está disponível a um agente
de IA, pela mesma interface.

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
