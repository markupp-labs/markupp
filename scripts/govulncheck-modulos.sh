#!/usr/bin/env bash
# Roda o govulncheck em cada módulo Go versionado no repositório e reprova se
# algum tiver vulnerabilidade alcançável pelo código. Verifica todos os módulos
# antes de sair, para mostrar todas as falhas de uma vez.
set -uo pipefail

GOVULNCHECK=golang.org/x/vuln/cmd/govulncheck@v1.8.0
RAIZ="$(git rev-parse --show-toplevel)"
reprovados=0

for gomod in $(git -C "$RAIZ" ls-files 'go.mod' '*/go.mod'); do
  modulo="$(dirname "$gomod")"
  if ! (cd "$RAIZ/$modulo" && go run "$GOVULNCHECK" ./...); then
    printf 'govulncheck reprovou o módulo %s\n' "$modulo" >&2
    reprovados=$((reprovados + 1))
  fi
done

[ "$reprovados" -eq 0 ]
