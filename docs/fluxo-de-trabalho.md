# Fluxo de trabalho

Como o código entra no repositório: branches, commits, pull requests e releases.

## Branches

- main: recebe só a dev, na consolidação de um release.
- dev: integração. Todo trabalho parte dela e volta para ela.
- Branches de trabalho: saem da dev com o prefixo do tipo de commit e uma descrição curta, como feat/log-do-notificador, fix/otel-1-47 ou docs/glossario-instancia. O prefixo é feat/, nunca feature/, e refact/, nunca refactor/, mesmo que merges antigos mostrem o contrário.

## Commits

Conventional commits em português, sem escopo entre parênteses: `feat: adiciona validação de upload`. Os tipos aceitos são feat, fix, docs, test, chore, refact, release e merge. Refatoração usa refact:, e o hook recusa refactor:.

- release: consolida a dev na main.
- merge: é o back-merge escrito à mão, de main para dev ou de dev para uma branch de trabalho.

Cada commit traz uma mudança lógica. Refatoração, feature e correção vão em commits separados, mesmo quando mexem no mesmo arquivo.

O hook commit-msg roda [scripts/valida-mensagem-de-commit.sh](../scripts/valida-mensagem-de-commit.sh) e também recusa emoji, travessão e atribuição de autoria a ferramenta (Co-Authored-By e afins). Mensagens que o próprio git gera, como `Merge branch` e `Revert`, passam. Instale os hooks no clone com `make hooks`. A CI roda o mesmo script em todos os commits de cada PR.

## Pull requests

O PR de trabalho mira a dev. Use o [template de PR](../.github/PULL_REQUEST_TEMPLATE.md), e issues seguem os [templates de issue](../.github/ISSUE_TEMPLATE/).

O repositório só aceita merge commit: squash e rebase estão desligados.

A main é protegida. Um PR para ela precisa de:

- uma aprovação, e um push novo descarta as aprovações anteriores;
- os checks Server (Go) tests e Commit conventions verdes, com a branch atualizada em relação à main;
- todas as conversas resolvidas.

Force push e remoção da main estão bloqueados. A dev não tem proteção configurada.

## Release

1. Abra um PR da dev para a main com o título `release: ...` e faça o merge com merge commit.
2. Traga a main de volta para a dev com um commit `merge:`, por exemplo `merge: traz o main da release da rc.10 de volta à dev`. Sem esse back-merge, as duas branches divergem.
3. Marque o release com uma tag semver com prefixo v, como v1.0.0 ou v1.0.0-rc.1. O [workflow de release](../.github/workflows/release.yml) recusa tag fora desse formato.

A tag publica as imagens no GHCR e o pacote markupp-mcp no npm. Tag de prerelease (com hífen) não move a tag latest da imagem e vai para a dist-tag next do npm.
