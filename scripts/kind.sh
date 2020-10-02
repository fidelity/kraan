#!/bin/bash
set -o errexit
# desired cluster name; default is "kind"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-k8s}"
KIND_PORT=${KIND_CLUSTER_PORT:-16443}
cat <<EOF | kind create cluster --name "${KIND_CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "127.0.0.1"
  apiServerPort: ${KIND_PORT}
kubeadmConfigPatches:
- |-
nodes:
- role: control-plane
- role: worker
EOF

kubectl config use-context kind-${KIND_CLUSTER_NAME}
