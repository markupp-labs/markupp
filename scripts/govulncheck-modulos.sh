#!/usr/bin/env bash
# Roda o govulncheck em cada módulo Go versionado no repositório e reprova se
# algum tiver vulnerabilidade alcançável pelo código. Verifica todos os módulos
# antes de sair, para mostrar todas as falhas de uma vez.
set -uo pipefail

GOVULNCHECK=golang.org/x/vuln/cmd/govulncheck@v1.8.0
RAIZ="$(git rev-parse --show-toplevel)"
verificados=0
reprovados=0

while IFS= read -r -d '' gomod; do
  modulo="$(dirname "$gomod")"
  verificados=$((verificados + 1))
  if ! (cd "$RAIZ/$modulo" && go run "$GOVULNCHECK" ./...); then
    printf 'govulncheck reprovou o módulo %s\n' "$modulo" >&2
    reprovados=$((reprovados + 1))
  fi
done < <(git -C "$RAIZ" ls-files -z 'go.mod' '*/go.mod')

if [ "$verificados" -eq 0 ]; then
  printf 'nenhum go.mod versionado em %s, esperado ao menos um módulo Go\n' "$RAIZ" >&2
  exit 1
fi
[ "$reprovados" -eq 0 ]
