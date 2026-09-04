#!/usr/bin/env bash
set -uo pipefail

VALIDADOR="$(cd "$(dirname "$0")" && pwd)/valida-mensagem-de-commit.sh"
falhas=0

verifica() {
  local esperado="$1" descricao="$2" mensagem="$3"
  local arquivo status
  arquivo="$(mktemp)"
  printf '%s\n' "$mensagem" >"$arquivo"
  "$VALIDADOR" "$arquivo" >/dev/null 2>&1
  status=$?
  rm -f "$arquivo"
  if [ "$status" -eq "$esperado" ]; then
    printf 'ok    %s\n' "$descricao"
    return 0
  fi
  printf 'FALHA %s (esperava saida %s, recebeu %s)\n' "$descricao" "$esperado" "$status"
  falhas=$((falhas + 1))
}

aceita() { verifica 0 "$1" "$2"; }
rejeita() { verifica 1 "$1" "$2"; }

aceita 'tipo valido com descricao' 'feat: adiciona validacao de upload'
aceita 'refact e um tipo aceito' 'refact: extrai funcao de parsing'
aceita 'merge commit nao e validado' 'Merge pull request #7 from markupp-labs/docs'
aceita 'revert nao e validado' 'Revert "feat: adiciona rota"'
aceita 'comentario do git e ignorado' "$(printf 'feat: adiciona rota\n# Co-Authored-By: Ferramenta <a@b.c>')"
rejeita 'refactor no lugar de refact' 'refactor: extrai funcao de parsing'
rejeita 'escopo entre parenteses' 'feat(api): adiciona rota de busca'
rejeita 'tipo fora da lista' 'melhoria: ajusta o layout'
rejeita 'assunto sem tipo' 'ajusta o layout'
rejeita 'tipo sem descricao' 'feat:'
rejeita 'emoji no assunto' 'feat: adiciona rota de busca com IA 🚀'
rejeita 'travessao no corpo' "$(printf 'feat: adiciona rota\n\nresolve o caso principal — o mais comum')"
rejeita 'trailer co-authored-by' "$(printf 'feat: adiciona rota\n\nCo-Authored-By: Ferramenta <a@b.c>')"
rejeita 'rodape de atribuicao' "$(printf 'feat: adiciona rota\n\nGenerated with alguma ferramenta')"

if [ "$falhas" -ne 0 ]; then
  printf '\n%s caso(s) falharam\n' "$falhas"
  exit 1
fi
printf '\ntodos os casos passaram\n'
