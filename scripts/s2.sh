#!/bin/bash

set -euo pipefail

function usage()
{
    set +x
    echo "USAGE: ${0##*/} [--debug] [--dry-run] [--toolkit]"
	echo "Install the GitOps Toolkit to Kubernetes cluster"
	echo "options:"
  echo "'--debug' for verbose output"
  echo "'--toolkit' to generate GitOps toolkit components"
  echo "'--dry-run' to generate yaml but not deploy to cluster. This option will retain temporary work directory"
}

function args() {
  debug=""
  toolkit=""
  dry_run=""

  arg_list=( "$@" )
  arg_count=${#arg_list[@]}
  arg_index=0
  while (( arg_index < arg_count )); do
    case "${arg_list[${arg_index}]}" in
          "--toolkit") toolkit=1;;
          "--dry-run") dry_run=1;;
          "--debug") debug=1;set -x;;
               "-h") usage; exit;;
           "--help") usage; exit;;
               "-?") usage; exit;;
        *) if [ "${arg_list[${arg_index}]:0:2}" == "--" ];then
               echo "invalid argument: ${arg_list[${arg_index}]}"
               usage; exit
           fi;
           break;;
    esac
    (( arg_index+=1 ))
  done
}

function toolkit_refresh() {
  tk install --export --components=source-controller > "${work_dir}"/gitops/gitops-original.yaml
}

function create_regcred() {
  local namespace=${1}
  jq -r '{auths: .auths}' ~/.docker/config.json > image_pull_secret.json
  kubectl -n ${namespace} delete  --ignore-not-found=true secret regcred 
  kubectl -n ${namespace} create secret generic regcred \
    --from-file=.dockerconfigjson=image_pull_secret.json \
    --type=kubernetes.io/dockerconfigjson
}

function install_helm() {
  # Install Helm Operator, already present on some systems, so check first if needed
  set +e
  kubectl -n cluster-addons get deployments flux-helm-operator
  if [ "$?" == "0" ] ; then
    echo "helm-operator already present"
    set -e
    return
  fi
  set -e
  echo "helm-operator not installed, installing"
  helm repo add fluxcd https://charts.fluxcd.io
  kubectl apply -f https://raw.githubusercontent.com/fluxcd/helm-operator/1.1.0/deploy/crds.yaml
  helm upgrade -i helm-operator fluxcd/helm-operator --namespace cluster-addons --set helm.versions=v3
}

args "$@"

base_dir="$(git rev-parse --show-toplevel)"
work_dir="$(mktemp -d -t global-config-XXXXXX)"

cp -rf "${base_dir}"/addons/gitops "${work_dir}"

if [ -n "${toolkit}" ] ; then
  toolkit_refresh
fi

if [ -n "${dry_run}" ] ; then
  echo "Yaml files are in ${work_dir}"
  exit 0
fi

kubectl apply -f "${work_dir}"/gitops/gitops.yaml
create_regcred gitops-system
kubectl apply -f "${base_dir}"/addons/addons-config/namespace.yaml
create_regcred addons-config
install_helm
kubectl apply -k "${base_dir}"/addons/addons-config
rm -rf "${work_dir}"