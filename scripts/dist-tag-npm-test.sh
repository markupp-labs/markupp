#!/usr/bin/env bash
set -uo pipefail

PICKER="$(cd "$(dirname "$0")" && pwd)/dist-tag-npm.sh"
failures=0

expect_tag() {
  local expected="$1" description="$2"
  shift 2
  local actual
  actual="$("$PICKER" "$@" 2>/dev/null)"
  if [ "$actual" = "$expected" ]; then
    printf 'ok    %s\n' "$description"
    return 0
  fi
  printf 'FAIL  %s (expected %s, got %s)\n' "$description" "$expected" "$actual"
  failures=$((failures + 1))
}

expect_rejection() {
  local description="$1"
  shift
  if "$PICKER" "$@" >/dev/null 2>&1; then
    printf 'FAIL  %s (expected a non-zero exit)\n' "$description"
    failures=$((failures + 1))
    return 0
  fi
  printf 'ok    %s\n' "$description"
}

expect_tag latest 'primeira publicacao estavel vai para latest' 1.0.0 ''
expect_tag latest 'estavel maior que a latest vai para latest' 1.1.0 1.0.0
expect_tag latest 'compara numeros e nao texto' 1.10.0 1.9.0
expect_tag latest-1.0 'correcao de linha antiga nao rebaixa a latest' 1.0.1 1.1.0
expect_tag latest-1.1 'versao igual a latest nao vira latest de novo' 1.1.0 1.1.0
expect_tag next 'prerelease vai para next' 1.2.0-rc.1 1.1.0
expect_tag next 'prerelease sem latest vai para next' 1.0.0-rc.1 ''
expect_rejection 'versao fora do semver reprova' v1.0.0 1.0.0
expect_rejection 'latest atual fora do semver reprova' 1.0.0 lixo
expect_rejection 'sem versao reprova'

if [ "$failures" -ne 0 ]; then
  printf '\n%s case(s) failed\n' "$failures"
  exit 1
fi
printf '\nall cases passed\n'
