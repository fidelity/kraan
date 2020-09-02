#!/bin/bash
set -o errexit
export MY_IP=$(ip -o route get to 8.8.8.8 | sed -n 's/.*src \([0-9.]\+\).*/\1/p')
# desired cluster name; default is "kind"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-k8s}"
export KUBECONFIG=$HOME/kind-${KIND_CLUSTER_NAME}.config

# create registry container unless it already exists
reg_name='registry'
reg_port='5001'
running="$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)"
if [ "${running}" != 'true' ]; then
  docker run -d --restart=always -p "${reg_port}:5000" --name "${reg_name}" registry:2
fi
reg_ip="$(docker inspect -f '{{.NetworkSettings.IPAddress}}' "${reg_name}")"

# create a cluster with the local registry enabled in containerd
cat <<EOF | kind create cluster --name "${KIND_CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "$MY_IP"
  apiServerPort: 16443
containerdConfigPatches: 
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
    endpoint = ["http://${reg_ip}:${reg_port}"]
kubeadmConfigPatches:
- |-
nodes:
- role: control-plane
- role: worker
- role: worker
EOF

kubectl config use-context kind-${KIND_CLUSTER_NAME}

echo "export KUBECONFIG=$HOME/kind-${KIND_CLUSTER_NAME}.config"
