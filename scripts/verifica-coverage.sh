#!/usr/bin/env bash
set -uo pipefail

PACOTES_SEM_AUTORIA_HUMANA='(^|/)(storage/gen|cmd/markupp)/'

percentual_de() {
  awk 'NR > 1 {
    total += $2
    if ($3 > 0) { cobertas += $2 }
  } END {
    if (total == 0) { print "0.0"; exit }
    printf "%.1f", cobertas * 100 / total
  }' "$1"
}

sem_codigo_gerado() {
  grep -vE "$PACOTES_SEM_AUTORIA_HUMANA" "$1"
}

atinge_meta() {
  awk -v atingido="$1" -v meta="$2" 'BEGIN { exit !(atingido + 0 >= meta + 0) }'
}

main() {
  local perfil="$1" meta="$2"
  local filtrado atingido
  filtrado="$(mktemp)"
  sem_codigo_gerado "$perfil" >"$filtrado"
  atingido="$(percentual_de "$filtrado")"
  rm -f "$filtrado"
  if atinge_meta "$atingido" "$meta"; then
    printf 'coverage de %s%% atinge a meta de %s%%\n' "$atingido" "$meta"
    return 0
  fi
  printf 'coverage de %s%% abaixo da meta de %s%%\n' "$atingido" "$meta" >&2
  exit 1
}

main "$@"
