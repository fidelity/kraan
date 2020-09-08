#!/bin/bash

set -euo pipefail

function usage()
{
    set +x
    echo "USAGE: ${0##*/} [--debug] [--dry-run] [--toolkit] [--deploy-kind]"
    echo "       [--kraan-image-pull-secret auto | <filename>] [--kraan-image-repo <repo-name>]"
    echo "       [--gitops-image-pull-secret auto | <filename>] [--gitops-image-repo <repo-name>]"
    echo "       [--gitops-proxy auto | <proxy-url>]"
    echo "Install the Kraan Addon Manager and gitops source controller to a Kubernetes cluster"
    echo "options:"
    echo "'--kraan-image-pull-secret' set to 'auto' to generate image pull secrets from ~/.docker/config.json or"
    echo "                            or supply name of file containing image pull secret defintion to apply."
    echo "                            the last element of the filename should also be the secret name, e.g."
    echo "                            filename /tmp/regcred.yaml should define a secret called 'regcred'"
    echo "'--gitops-image-pull-secret' as above for gitops components"
    echo "'--kraan-image-repo' provide image repository to use for Kraan, docker.pkg.github.com/"
    echo "'--gitops-image-repo' provide image repository to use for gitops components, defaults to docker.io/fluxcd"
    echo "'--gitops-proxy' set to 'auto' to generate proxy setting for source controller using value of HTTPS_PROXY environmental variable"
    echo "                 or supply the proxy url to use."
    echo "'--deploy-kind' to create a kind cluster and use that to deploy to. Otherwise it will deploy to an existing cluster."
    echo "                The KUBECONFIG environmental variable or ~/.kube/config should be set to a cluster admin user for the"
    echo "                cluster you want to use. This cluster must be running API version 16 or greater."
    echo "'--toolkit' to generate GitOps toolkit components"
    echo "'--debug' for verbose output"
    echo "'--dry-run' to generate yaml but not deploy to cluster. This option will retain temporary work directory"
    echo "The environmental variable GIT_USER and GIT_CREDENTIALS must be set to the git user and credentials respectively"
}

function args() {
  debug=""
  toolkit=""
  dry_run=""
  deploy_kind=""
  gitops_repo=""
  kraan_repo=""
  kraan_regcred=""
  gitops_regcred=""
  gitops_proxy=""

  arg_list=( "$@" )
  arg_count=${#arg_list[@]}
  arg_index=0
  while (( arg_index < arg_count )); do
    case "${arg_list[${arg_index}]}" in
          "--toolkit") toolkit=1;;
          "--deploy-kind") deploy_kind=1;;
          "--kraan-image-pull-secret") (( arg_index+=1 )); kraan_regcred="${arg_list[${arg_index}]}";;
          "--gitops-image-pull-secret") (( arg_index+=1 )); gitops_regcred="${arg_list[${arg_index}]}";toolkit=1;;
          "--gitops-proxy") (( arg_index+=1 )); gitops_proxy="${arg_list[${arg_index}]}";toolkit=1;;
          "--kraan-image-repo") (( arg_index+=1 )); kraan_repo="${arg_list[${arg_index}]}";;
          "--gitops-image-repo") (( arg_index+=1 )); gitops_repo="${arg_list[${arg_index}]}";toolkit=1;;
          "--dry-run") dry_run="--dry-run";;
          "--debug") set -x;;
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
  if [ -z "${GIT_USER:-}" ] ; then 
    echo "GIT_USER must be set to the git user name"
    usage; exit 1
  fi
  if [ -z "${GIT_CREDENTIALS:-}" ] ; then
      echo "GIT_CREDENTIALS must be set to the git user's password or token"
      usage; exit 1
  fi
}

function create_secrets {
  local base64_user="$(echo -n "${GIT_USER}" | base64 -w 0)"
  local base64_creds="$(echo -n "${GIT_CREDENTIALS}" | base64 -w 0)"
  sed s/GIT_USER/${base64_user}/ "${base_dir}/"testdata/templates/template-http.yaml | \
  sed s/GIT_CREDENTIALS/${base64_creds}/ > "${work_dir}"/kraan-http.yaml
}

function toolkit_refresh() {
  local gitops_repo_arg=""
  local gitops_regcred_arg=""
  local secret_name=""
  if [ -n "${gitops_repo}" ] ; then
    gitops_repo_arg="--registry ${gitops_repo}"
  fi
  if [ -n "${gitops_regcred}" ] ; then
    secret_name="regcred"
    if [ "${gitops_regcred}" != "auto" ] ; then
      secret_name=$(basename "${gitops_regcred}" | cut -f1 -d.)
    fi
    gitops_regcred_arg="--image-pull-secret ${secret_name}"
  fi
  gotk install --export --components=source-controller ${gitops_repo_arg} ${gitops_regcred_arg} > "${work_dir}"/gitops/gitops.yaml
  if [ -n "${dry_run}" ] ; then
    echo "yaml for gitops toolkit is in ${work_dir}/gitops/gitops.yaml"
  fi
  if [ -n "${gitops_proxy}" ] ; then
    local gitops_proxy_url="${gitops_proxy}"
    if [ "${gitops_proxy}" == "auto" ] ; then
      gitops_proxy_url="${HTTPS_PROXY}"
    fi
    cp "${work_dir}"/gitops/gitops.yaml "${work_dir}"/gitops/gitops-orignal.yaml
    awk '/metadata.namespace:/{ print "        - name: HTTPS_PROXY\n          value: ${gitops_proxy_url}\n        - name: NO_PROXY\n         value: 10.0.0.0/8"}1' \
        "${work_dir}"/gitops/gitops-orignal.yaml > "${work_dir}"/gitops/gitops.yaml
  fi
}

function create_regcred() {
  local namespace="${1}"
  local auto_file="${2}"
  if [ "${auto_file}" == "auto" ] ; then
    if [ -n "${dry_run}" ] ; then
      return
    fi
    jq -r '{auths: .auths}' ~/.docker/config.json > "${work_dir}"/image_pull_secret.json
    kubectl -n "${namespace}" delete  --ignore-not-found=true secret regcred 
    kubectl -n "${namespace}" create secret generic regcred \
      --from-file=.dockerconfigjson="${work_dir}"/image_pull_secret.json \
      --type=kubernetes.io/dockerconfigjson
  else
    if [ -f "${auto_file}" ] ; then
      kubectl apply ${dry_run} -f "${auto_file} --namespace ${namespace}"
    else
      echo "File: '${auto_file}' not found"
      exit 1
    fi
  fi
}

function deploy_kraan_mgr() {
  cp -rf "${base_dir}"/testdata/addons/kraan/manager "${work_dir}"
  if [ -n "${kraan_repo}" ] ; then
    sed -i "s#image\:\ docker.pkg.github.com/addons-mgmt#image\:\ ${kraan_repo}#" "${work_dir}"/manager/deployment.yaml
  fi
  if [ -n "${kraan_regcred}" ] ; then
    local secret_name="regcred"
    if [ "${kraan_regcred}" != "auto" ] ; then
      secret_name=$(basename "${kraan_regcred}" | cut -f1 -d.)
    fi
    cp "${work_dir}"/manager/deployment.yaml "${work_dir}"/deployment-orignal.yaml
    awk '/containers:/{ print "      imagePullSecrets:\n      - name: ${secret_name}"}1'  "${work_dir}"/deployment-orignal.yaml > "${work_dir}"/manager/deployment.yaml
  fi
  kubectl apply ${dry_run} -f "${work_dir}"/manager
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
  kubectl apply ${dry_run} -f https://raw.githubusercontent.com/fluxcd/helm-operator/1.1.0/deploy/crds.yaml
  helm upgrade ${dry_run} -i helm-operator fluxcd/helm-operator --namespace kraan --set helm.versions=v3
}

args "$@"

base_dir="$(git rev-parse --show-toplevel)"
work_dir="$(mktemp -d -t kraan-XXXXXX)"

if [ -n "${deploy_kind}" ] ; then
  KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-k8s}"
  "${base_dir}"/scripts/kind-with-registry.sh
  export KUBECONFIG=$HOME/kind-${KIND_CLUSTER_NAME}.config
fi

create_secrets

cp -rf "${base_dir}"/testdata/addons/gitops "${work_dir}"

if [ -n "${toolkit}" ] ; then
  toolkit_refresh
fi

if [ -n "${dry_run}" ] ; then
  echo "yaml for gitops toolkit is in ${work_dir}/gitops/gitops.yaml"
fi

kubectl apply ${dry_run} -f "${work_dir}"/gitops/gitops.yaml
kubectl apply ${dry_run} -f "${work_dir}"/kraan-http.yaml

if [ -n "${gitops_regcred}" ] ; then
  create_regcred gitops-system "${gitops_regcred}"
fi

kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/addons-source.yaml

kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/kraan/namespace.yaml

if [ -n "${kraan_regcred}" ] ; then
  create_regcred kraan "${kraan_regcred}"
fi

install_helm

kubectl apply ${dry_run} -k "${base_dir}"/config/crd
kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/kraan/rbac

deploy_kraan_mgr

# Create namespaces for each addon layer
kubectl apply ${dry_run} -f "${base_dir}"/testdata/namespaces.yaml

kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/addons.yaml
if [ -z "${dry_run}" ] ; then
  rm -rf "${work_dir}"
fi
