#!/usr/bin/env bash
set -uo pipefail

VERIFICADOR="$(cd "$(dirname "$0")" && pwd)/govulncheck-modulos.sh"
falhas=0

# repo_com_dois_modulos monta um repositório git descartável com os módulos a e
# b, e um go falso no PATH que registra onde rodou e reprova o módulo em
# FALHA_EM, no lugar do govulncheck de verdade.
repo_com_dois_modulos() {
  local raiz="$1"
  git init -q "$raiz"
  mkdir -p "$raiz/a" "$raiz/b" "$raiz/bin"
  printf 'module a\n' >"$raiz/a/go.mod"
  printf 'module b\n' >"$raiz/b/go.mod"
  git -C "$raiz" add a/go.mod b/go.mod
  cat >"$raiz/bin/go" <<'EOF'
#!/usr/bin/env bash
basename "$PWD" >>"$REGISTRO"
[ "$(basename "$PWD")" != "${FALHA_EM:-}" ]
EOF
  chmod +x "$raiz/bin/go"
}

verifica() {
  local esperado="$1" descricao="$2" falha_em="$3"
  local raiz status
  raiz="$(mktemp -d)"
  repo_com_dois_modulos "$raiz"
  (cd "$raiz" && PATH="$raiz/bin:$PATH" REGISTRO="$raiz/registro" FALHA_EM="$falha_em" "$VERIFICADOR") >/dev/null 2>&1
  status=$?
  local rodou
  rodou="$(sort "$raiz/registro" 2>/dev/null | tr '\n' ' ')"
  rm -rf "$raiz"
  if [ "$status" -eq "$esperado" ] && [ "$rodou" = "a b " ]; then
    printf 'ok    %s\n' "$descricao"
    return 0
  fi
  printf 'FALHA %s (esperava saida %s em a e b, recebeu %s em "%s")\n' "$descricao" "$esperado" "$status" "$rodou"
  falhas=$((falhas + 1))
}

verifica 0 'todos os modulos sem vulnerabilidade passam' ''
verifica 1 'um modulo com vulnerabilidade reprova' 'b'
verifica 1 'reprovar o primeiro modulo ainda verifica o segundo' 'a'

if [ "$falhas" -ne 0 ]; then
  printf '\n%s caso(s) falharam\n' "$falhas"
  exit 1
fi
printf '\ntodos os casos passaram\n'
