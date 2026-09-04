#!/usr/bin/env bash
set -uo pipefail

TIPOS_ACEITOS='feat|fix|docs|test|chore|refact'
FAIXAS_DE_EMOJI='[\x{1F000}-\x{1FAFF}\x{2600}-\x{27BF}\x{2B00}-\x{2BFF}\x{FE0F}]'

recusa() {
  printf 'mensagem de commit rejeitada: %s\n' "$1" >&2
  exit 1
}

sem_comentarios() {
  grep -v '^#' "$1"
}

assunto_de() {
  sem_comentarios "$1" | sed '/^[[:space:]]*$/d' | head -1
}

e_gerado_pelo_git() {
  printf '%s' "$1" | grep -qE '^(Merge|Revert) '
}

valida_tipo() {
  if printf '%s' "$1" | grep -qE '^refactor(\([^)]*\))?:'; then
    recusa "use 'refact:' no lugar de 'refactor:'. assunto recebido: $1"
  fi
  if printf '%s' "$1" | grep -qE "^($TIPOS_ACEITOS)(\([^)]*\))?: .+"; then
    return 0
  fi
  recusa "esperado '<tipo>: <descricao>' com tipo em ($TIPOS_ACEITOS). assunto recebido: $1"
}

valida_ausencia_de_escopo() {
  if printf '%s' "$1" | grep -qE '^[a-z]+\([^)]*\):'; then
    recusa "escopo entre parenteses nao e usado neste projeto. assunto recebido: $1"
  fi
}

valida_ausencia_de_emoji() {
  if printf '%s' "$1" | grep -qP "$FAIXAS_DE_EMOJI"; then
    recusa "a mensagem contem emoji, que nao e usado em nenhum texto deste projeto"
  fi
}

valida_ausencia_de_travessao() {
  if printf '%s' "$1" | grep -q '—'; then
    recusa "a mensagem contem travessao, use virgula, dois-pontos ou parenteses"
  fi
}

valida_ausencia_de_atribuicao() {
  if printf '%s' "$1" | grep -qiE 'co-authored-by:|generated with|noreply@anthropic'; then
    recusa "a mensagem credita autoria a uma ferramenta, o autor do trabalho e a pessoa"
  fi
}

main() {
  local assunto mensagem
  assunto="$(assunto_de "$1")"
  if e_gerado_pelo_git "$assunto"; then
    exit 0
  fi
  mensagem="$(sem_comentarios "$1")"
  valida_tipo "$assunto"
  valida_ausencia_de_escopo "$assunto"
  valida_ausencia_de_emoji "$mensagem"
  valida_ausencia_de_travessao "$mensagem"
  valida_ausencia_de_atribuicao "$mensagem"
}

main "$@"
