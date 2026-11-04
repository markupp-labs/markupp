#!/usr/bin/env bash
# Escolhe a dist-tag do npm para publicar VERSAO do markupp-mcp, dado o que a dist-tag
# latest aponta hoje (vazio quando o pacote ainda não existe no npm).
#   prerelease                      -> next
#   estável maior que a latest      -> latest
#   estável igual ou menor          -> latest-MAJOR.MINOR, para não rebaixar o npx
# Uso: dist-tag-npm.sh VERSION [CURRENT_LATEST]
set -euo pipefail

SEMVER='^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$'
STABLE='^[0-9]+\.[0-9]+\.[0-9]+$'

require_format() {
  local name="$1" value="$2" format="$3" example="$4"
  if printf '%s' "$value" | grep -qE "$format"; then
    return 0
  fi
  printf '%s %q is malformed, expected something like %s\n' "$name" "$value" "$example" >&2
  exit 1
}

greater_than() {
  local version="$1" current="$2"
  [ "$version" != "$current" ] &&
    [ "$(printf '%s\n%s\n' "$current" "$version" | sort -V | tail -n 1)" = "$version" ]
}

version="${1:-}"
current="${2:-}"
require_format version "$version" "$SEMVER" '1.0.0 or 1.0.0-rc.1, without the v'
if [ -n "$current" ]; then
  require_format 'current latest' "$current" "$STABLE" '1.0.0'
fi

if [[ "$version" == *-* ]]; then
  echo next
elif [ -z "$current" ] || greater_than "$version" "$current"; then
  echo latest
else
  echo "latest-${version%.*}"
fi
