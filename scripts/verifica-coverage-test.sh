#!/usr/bin/env bash
set -uo pipefail

VERIFICADOR="$(cd "$(dirname "$0")" && pwd)/verifica-coverage.sh"
falhas=0

perfil_com() {
  local arquivo
  arquivo="$(mktemp)"
  printf 'mode: set\n%s\n' "$1" >"$arquivo"
  printf '%s' "$arquivo"
}

verifica() {
  local esperado="$1" descricao="$2" meta="$3" linhas="$4"
  local perfil status
  perfil="$(perfil_com "$linhas")"
  "$VERIFICADOR" "$perfil" "$meta" >/dev/null 2>&1
  status=$?
  rm -f "$perfil"
  if [ "$status" -eq "$esperado" ]; then
    printf 'ok    %s\n' "$descricao"
    return 0
  fi
  printf 'FALHA %s (esperava saida %s, recebeu %s)\n' "$descricao" "$esperado" "$status"
  falhas=$((falhas + 1))
}

aceita() { verifica 0 "$1" "$2" "$3"; }
rejeita() { verifica 1 "$1" "$2" "$3"; }

aceita 'tudo coberto passa na meta' 90 \
  'internal/notes/notes.go:10.1,12.2 4 1'
rejeita 'metade coberta reprova na meta' 90 \
  "$(printf 'internal/notes/notes.go:10.1,12.2 4 1\ninternal/notes/notes.go:14.1,16.2 4 0')"
aceita 'meta exatamente atingida passa' 50 \
  "$(printf 'internal/notes/notes.go:10.1,12.2 4 1\ninternal/notes/notes.go:14.1,16.2 4 0')"
aceita 'codigo gerado pelo sqlc nao conta' 90 \
  "$(printf 'internal/notes/notes.go:10.1,12.2 4 1\ninternal/storage/gen/notes.sql.go:1.1,99.2 80 0')"
aceita 'fiacao do main nao conta' 90 \
  "$(printf 'internal/notes/notes.go:10.1,12.2 4 1\ncmd/markupp/main.go:1.1,50.2 40 0')"
rejeita 'perfil sem nenhuma linha reprova' 90 ''

if [ "$falhas" -ne 0 ]; then
  printf '\n%s caso(s) falharam\n' "$falhas"
  exit 1
fi
printf '\ntodos os casos passaram\n'
