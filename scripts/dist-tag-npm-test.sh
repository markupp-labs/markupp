#!/usr/bin/env bash
set -uo pipefail

ESCOLHEDOR="$(cd "$(dirname "$0")" && pwd)/dist-tag-npm.sh"
falhas=0

escolhe() {
  local esperado="$1" descricao="$2"
  shift 2
  local recebido
  recebido="$("$ESCOLHEDOR" "$@" 2>/dev/null)"
  if [ "$recebido" = "$esperado" ]; then
    printf 'ok    %s\n' "$descricao"
    return 0
  fi
  printf 'FALHA %s (esperava %s, recebeu %s)\n' "$descricao" "$esperado" "$recebido"
  falhas=$((falhas + 1))
}

rejeita() {
  local descricao="$1"
  shift
  if "$ESCOLHEDOR" "$@" >/dev/null 2>&1; then
    printf 'FALHA %s (esperava saida diferente de 0)\n' "$descricao"
    falhas=$((falhas + 1))
    return 0
  fi
  printf 'ok    %s\n' "$descricao"
}

escolhe latest 'primeira publicacao estavel vai para latest' 1.0.0 ''
escolhe latest 'estavel maior que a latest vai para latest' 1.1.0 1.0.0
escolhe latest 'compara numeros e nao texto' 1.10.0 1.9.0
escolhe latest-1.0 'correcao de linha antiga nao rebaixa a latest' 1.0.1 1.1.0
escolhe latest-1.1 'versao igual a latest nao vira latest de novo' 1.1.0 1.1.0
escolhe next 'prerelease vai para next' 1.2.0-rc.1 1.1.0
escolhe next 'prerelease sem latest vai para next' 1.0.0-rc.1 ''
rejeita 'versao fora do semver reprova' v1.0.0 1.0.0
rejeita 'latest atual fora do semver reprova' 1.0.0 lixo
rejeita 'sem versao reprova'

if [ "$falhas" -ne 0 ]; then
  printf '\n%s caso(s) falharam\n' "$falhas"
  exit 1
fi
printf '\ntodos os casos passaram\n'
