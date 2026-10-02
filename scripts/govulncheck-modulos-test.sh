#!/usr/bin/env bash
set -uo pipefail

VERIFICADOR="$(cd "$(dirname "$0")" && pwd)/govulncheck-modulos.sh"
ESPERADO_GO='run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...'
falhas=0

# repo_com_modulos monta um repositório git descartável com os módulos pedidos,
# inclusive com espaço no caminho, e um go falso no PATH que registra onde rodou
# e com quais argumentos, e reprova o módulo em FALHA_EM, no lugar do
# govulncheck de verdade.
repo_com_modulos() {
  local raiz="$1"
  shift
  git init -q "$raiz"
  mkdir -p "$raiz/bin"
  for modulo in "$@"; do
    mkdir -p "$raiz/$modulo"
    printf 'module m\n' >"$raiz/$modulo/go.mod"
    git -C "$raiz" add "$modulo/go.mod"
  done
  cat >"$raiz/bin/go" <<'EOF'
#!/usr/bin/env bash
printf '%s|%s\n' "$(basename "$PWD")" "$*" >>"$REGISTRO"
[ "$(basename "$PWD")" != "${FALHA_EM:-}" ]
EOF
  chmod +x "$raiz/bin/go"
}

roda() {
  local raiz="$1" falha_em="$2"
  (cd "$raiz" && PATH="$raiz/bin:$PATH" REGISTRO="$raiz/registro" FALHA_EM="$falha_em" "$VERIFICADOR") >/dev/null 2>&1
}

relata() {
  local ok="$1" descricao="$2" detalhe="$3"
  if [ "$ok" = sim ]; then
    printf 'ok    %s\n' "$descricao"
    return 0
  fi
  printf 'FALHA %s (%s)\n' "$descricao" "$detalhe"
  falhas=$((falhas + 1))
}

verifica() {
  local esperado="$1" descricao="$2" falha_em="$3"
  local raiz status rodou ok=nao
  raiz="$(mktemp -d)"
  repo_com_modulos "$raiz" a 'b c'
  roda "$raiz" "$falha_em"
  status=$?
  rodou="$(sort "$raiz/registro" 2>/dev/null | tr '\n' ';')"
  rm -rf "$raiz"
  [ "$status" -eq "$esperado" ] && [ "$rodou" = "a|$ESPERADO_GO;b c|$ESPERADO_GO;" ] && ok=sim
  relata "$ok" "$descricao" "esperava saida $esperado, recebeu $status com chamadas '$rodou'"
}

verifica_sem_modulo() {
  local raiz status ok=nao
  raiz="$(mktemp -d)"
  repo_com_modulos "$raiz"
  roda "$raiz" ''
  status=$?
  rm -rf "$raiz"
  [ "$status" -ne 0 ] && ok=sim
  relata "$ok" 'repositorio sem modulo go reprova em vez de passar calado' "recebeu saida $status"
}

verifica 0 'todos os modulos sem vulnerabilidade passam' ''
verifica 1 'um modulo com vulnerabilidade reprova' 'b c'
verifica 1 'reprovar o primeiro modulo ainda verifica o segundo' 'a'
verifica_sem_modulo

if [ "$falhas" -ne 0 ]; then
  printf '\n%s caso(s) falharam\n' "$falhas"
  exit 1
fi
printf '\ntodos os casos passaram\n'
