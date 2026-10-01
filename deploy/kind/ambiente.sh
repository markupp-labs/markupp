#!/usr/bin/env bash
# Ambiente da stack dev num kind com os requisitos do cluster de produção.
#
#   up      sobe o cluster, o painel Headlamp e a stack dev, e deixa de pé
#   painel  abre o Headlamp em http://localhost:4466 e mostra o token de acesso
#   down    apaga o cluster e o estado do Pulumi do ambiente de pé
#   smoke   sobe, testa a API por https e apaga tudo, passe ou falhe; é o do CI
set -euo pipefail

REPO="$(cd "$(dirname "$0")/../.." && pwd)"
STACKS="$REPO/deploy/stacks"
IMAGE=markupp:dev
URL=https://markupp.local:8443
AMBIENTE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/markupp-kind"
PAINEL_PORTA=4466

export CLUSTER=markupp-dev
export PULUMI_CONFIG_PASSPHRASE=""

# usa_diretorio guarda kubeconfig e estado do Pulumi em $1, longe do
# ~/.kube/config de quem roda.
usa_diretorio() {
  mkdir -p "$1/state"
  export KUBECONFIG="$1/kubeconfig"
  export PULUMI_BACKEND_URL="file://$1/state"
}

recusa_cluster_existente() {
  if kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
    echo "o cluster $CLUSTER já existe; rode make kind-down antes" >&2
    exit 1
  fi
}

sobe_cluster() {
  docker build -q -t "$IMAGE" "$REPO/markupp" >/dev/null
  "$REPO/deploy/kind/prepara-cluster.sh"
  kind load docker-image "$IMAGE" --name "$CLUSTER"
}

aplica_stack() {
  cd "$STACKS"
  pulumi stack select dev --create --non-interactive
  pulumi up --yes --non-interactive
}

instala_painel() {
  helm install headlamp headlamp --repo https://kubernetes-sigs.github.io/headlamp/ \
    --namespace headlamp --create-namespace --wait --timeout 10m
}

curl_api() {
  curl -sfk --resolve markupp.local:8443:127.0.0.1 "$@"
}

testa_api() {
  for _ in $(seq 1 30); do
    if curl_api "$URL/healthz"; then
      echo
      curl_api -X POST "$URL/notes" -H 'Content-Type: application/json' -d '{"path":"smoke.md","content":"oi"}'
      echo
      return 0
    fi
    sleep 5
  done
  kubectl -n markupp get all,certificate,gateway,httproute
  return 1
}

cmd_up() {
  recusa_cluster_existente
  usa_diretorio "$AMBIENTE_DIR"
  sobe_cluster
  instala_painel
  aplica_stack
  echo "API de pé: curl -k --resolve markupp.local:8443:127.0.0.1 $URL/healthz"
  echo "painel: make kind-painel"
}

cmd_painel() {
  usa_diretorio "$AMBIENTE_DIR"
  echo "token para entrar no Headlamp:"
  kubectl create token headlamp --namespace headlamp --duration 24h
  echo "painel em http://localhost:$PAINEL_PORTA, Ctrl+C para fechar"
  kubectl port-forward --namespace headlamp service/headlamp "$PAINEL_PORTA:80"
}

cmd_down() {
  kind delete cluster --name "$CLUSTER"
  rm -rf "$AMBIENTE_DIR"
}

cmd_smoke() {
  recusa_cluster_existente
  SMOKE_DIR="$(mktemp -d)"
  trap 'kind delete cluster --name "$CLUSTER" >/dev/null 2>&1 || true; rm -rf "$SMOKE_DIR"' EXIT
  usa_diretorio "$SMOKE_DIR"
  sobe_cluster
  aplica_stack
  testa_api
  echo "smoke passou"
}

case "${1:-}" in
  up) cmd_up ;;
  painel) cmd_painel ;;
  down) cmd_down ;;
  smoke) cmd_smoke ;;
  *)
    echo "subcomando '${1:-}' desconhecido, esperado up, painel, down ou smoke" >&2
    exit 2
    ;;
esac
