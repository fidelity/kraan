#!/bin/bash
set -o errexit
export MY_IP=$(ip -o route get to 8.8.8.8 | sed -n 's/.*src \([0-9.]\+\).*/\1/p')
# desired cluster name; default is "kind"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-k8s}"

cat <<EOF | kind create cluster --name "${KIND_CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "$MY_IP"
  apiServerPort: 16443
kubeadmConfigPatches:
- |-
nodes:
- role: control-plane
- role: worker
EOF

kubectl config use-context kind-${KIND_CLUSTER_NAME}
