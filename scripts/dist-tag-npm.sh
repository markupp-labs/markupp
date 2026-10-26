#!/usr/bin/env bash
# Escolhe a dist-tag do npm para publicar VERSAO do markupp-mcp, dado o que a dist-tag
# latest aponta hoje (vazio quando o pacote ainda não existe no npm).
#   prerelease                      -> next
#   estável maior que a latest      -> latest
#   estável igual ou menor          -> latest-MAJOR.MINOR, para não rebaixar o npx
# Uso: dist-tag-npm.sh VERSAO [LATEST_ATUAL]
set -euo pipefail

SEMVER='^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$'
ESTAVEL='^[0-9]+\.[0-9]+\.[0-9]+$'

exige_formato() {
  local nome="$1" valor="$2" formato="$3" exemplo="$4"
  if printf '%s' "$valor" | grep -qE "$formato"; then
    return 0
  fi
  printf '%s %q fora do formato, esperado algo como %s\n' "$nome" "$valor" "$exemplo" >&2
  exit 1
}

maior_que() {
  local versao="$1" atual="$2"
  [ "$versao" != "$atual" ] &&
    [ "$(printf '%s\n%s\n' "$atual" "$versao" | sort -V | tail -n 1)" = "$versao" ]
}

versao="${1:-}"
atual="${2:-}"
exige_formato versao "$versao" "$SEMVER" '1.0.0 ou 1.0.0-rc.1, sem o v'
if [ -n "$atual" ]; then
  exige_formato 'latest atual' "$atual" "$ESTAVEL" '1.0.0'
fi

if [[ "$versao" == *-* ]]; then
  echo next
elif [ -z "$atual" ] || maior_que "$versao" "$atual"; then
  echo latest
else
  echo "latest-${versao%.*}"
fi
