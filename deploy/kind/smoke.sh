#!/usr/bin/env bash
# Smoke da stack dev: sobe um kind com os requisitos do cluster do IFSC, aplica
# a stack dev do Pulumi, testa a API por https pelo Gateway e apaga tudo no fim,
# passe ou falhe. O CI roda este mesmo script.
set -euo pipefail

REPO="$(cd "$(dirname "$0")/../.." && pwd)"
STACKS="$REPO/deploy/stacks"
IMAGE=markupp:dev
URL=https://markupp.local:8443
WORKDIR="$(mktemp -d)"

export CLUSTER=markupp-dev
export KUBECONFIG="$WORKDIR/kubeconfig"
export PULUMI_BACKEND_URL="file://$WORKDIR/state"
export PULUMI_CONFIG_PASSPHRASE=""

limpa() {
  kind delete cluster --name "$CLUSTER" >/dev/null 2>&1 || true
  rm -rf "$WORKDIR"
}

sobe_cluster() {
  docker build -q -t "$IMAGE" "$REPO/markupp" >/dev/null
  "$REPO/deploy/kind/prepara-cluster.sh"
  kind load docker-image "$IMAGE" --name "$CLUSTER"
}

aplica_stack() {
  mkdir -p "$WORKDIR/state"
  cd "$STACKS"
  pulumi stack select dev --create --non-interactive
  pulumi up --yes --non-interactive
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

trap limpa EXIT
sobe_cluster
aplica_stack
testa_api
echo "smoke passou"
