#!/usr/bin/env bash
# Sobe o cluster kind da stack dev com os pré-requisitos que o cluster de produção
# tem: Gateway API, Cilium e cert-manager.
set -euo pipefail

CLUSTER="${CLUSTER:-markupp-dev}"
NAMESPACE="${NAMESPACE:-markupp}"
DIR="$(cd "$(dirname "$0")" && pwd)"
GATEWAY_API_VERSION=v1.6.1
CILIUM_VERSION=1.20.2
CERT_MANAGER_VERSION=v1.21.2

cria_cluster() {
  kind create cluster --name "$CLUSTER" --config "$DIR/cluster.yaml"
  kubectl apply --server-side -f \
    "https://github.com/kubernetes-sigs/gateway-api/releases/download/$GATEWAY_API_VERSION/standard-install.yaml"
}

instala_cilium() {
  helm install cilium cilium --repo https://helm.cilium.io --version "$CILIUM_VERSION" \
    --namespace kube-system --values "$DIR/cilium-values.yaml" \
    --set k8sServiceHost="$CLUSTER-control-plane" --wait --timeout 10m
}

instala_cert_manager() {
  kubectl apply -f \
    "https://github.com/cert-manager/cert-manager/releases/download/$CERT_MANAGER_VERSION/cert-manager.yaml"
  liga_gateway_no_cert_manager
  kubectl wait --for=condition=Available --timeout=10m deployment --all -n cert-manager
}

# liga_gateway_no_cert_manager faz o cert-manager emitir certificado a partir
# da anotação do Gateway, como o controller de produção roda com --enable-gateway-api.
liga_gateway_no_cert_manager() {
  kubectl -n cert-manager patch deployment cert-manager --type=json -p \
    '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--enable-gateway-api"}]'
}

cria_cluster
instala_cilium
instala_cert_manager
kubectl create namespace "$NAMESPACE"
