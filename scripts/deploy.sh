#!/bin/bash

export GIT_URL="${GIT_URL}"
export GIT_USER="${GIT_USER}"
export GIT_CREDENTIALS="${GIT_CREDENTIALS:-$GIT_TOKEN}"

set -euo pipefail

function usage() {
    set +x
    cat <<EOF
USAGE: ${0##*/} [--debug] [--dry-run] [--toolkit] [--deploy-kind] [--testdata] [--helm <upgrade| install>]
       [--kraan-image-reg <registry name>] [--kraan-image-repo <repo-name>] [--kraan-image-tag] [--kraan-dev]
       [--kraan-image-pull-secret auto | <filename>] [--gitops-image-pull-secret auto | <filename>]
       [--gitops-image-reg <repo-name>] [--kraan-loglevel N] [--prometheus <namespace>] [--values-files <file names>]
       [--gitops-proxy auto | <proxy-url>] [--git-url <git-repo-url>] [--no-git-auth]
       [--git-user <git_username>] [--git-token <git_token_or_password>]

Install the Kraan Addon Manager and gitops source controller to a Kubernetes cluster

Options:
  '--helm' perform helm upgrade or install, if you don't specify either script will not deploy helm chart.
  '--kraan-image-pull-secret' set to 'auto' to generate image pull secrets from ~/.docker/config.json
                              or supply name of file containing image pull secret defintion to apply.
                              The secret should be called 'kraan-regcred'.
  '--kraan-image-reg'   provide image registry to use for Kraan, defaults to empty for docker hub
  '--kraan-image-repo'  provide image repository prefix to use for Kraan, defaults to 'kraan' for docker hub org.
  '--kraan-image-name'  the name of the kraan image to use, defaults to kraan-controller.
  '--kraan-tag'         the tag of the kraan image to use.
  '--kraan-loglevel'    loglevel to use for kraan controller, 0 for info, 1 for debug, 2 for trace.
  '--kraan-dev'         select development mode, makes pod filesystem writable for debugging purposes.

  '--gitops-image-reg'  provide image registry to use for gitops toolkit components, defaults to 'ghcr.io'.
  '--gitops-image-pull-secret' set to 'auto' to generate image pull secrets from ~/.docker/config.json
                               or supply name of file containing image pull secret defintion to apply.
                               The secret should be called 'gotk-regcred'.
  '--gitops-proxy'    set to 'auto' to generate proxy setting for gotk components using value of HTTPS_PROXY
                      environment variable or supply the proxy url to use.
  '--values-files'    provide a comma separated list of yaml files containing values you want to set.
                      see samples directory for examples.

  '--deploy-kind' create a new kind cluster and deploy to it. Otherwise the script will deploy to an existing
                  cluster. The KUBECONFIG environmental variable or ~/.kube/config should be set to a cluster
                  admin user for the cluster you want to use. This cluster must be running API version 16 or
                  greater.
  '--prometheus'  install promethus stack in specified namespace
  '--testdata'    deploy testdata comprising addons layers and source controller custom resources to the target cluster.
  '--git-url'     set the URL for the git repository from which Kraan should pull AddonsLayer configs.
  '--git-user'    set (or override) the GIT_USER environment variables.
  '--git-token'   set (or override) the GIT_CREDENTIALS environment variables.
  '--no-git-auth' to indicate that no authorization is required to access git repository'
  '--toolkit'     to generate GitOps toolkit components.
  '--debug'       for verbose output.
  '--dry-run'     to generate yaml but not deploy to cluster. This option will retain temporary work directory.
EOF
}

function check_git_credentials() {
  echo "GIT_URL : ${GIT_URL}"
  echo "GIT_USER: ${GIT_USER}"
  echo "GIT_CRED: ${GIT_CREDENTIALS} "
  if [ -z "${GIT_USER}" ]; then
    echo "Getting GIT_USER from git config"
    GIT_USER=$(git config user.name)
  fi
  if [ -z "$GIT_CREDENTIALS" ]; then
    get_git_credentials_from_git
  fi
  echo "GIT_USER: ${GIT_USER}"
  echo "GIT_CRED: ${GIT_CREDENTIALS} "
}

function get_git_credentials_from_git() {
  echo "Getting GIT_CREDENTIALS from git credential fill for $GIT_URL"
  local CREDS=$(git credential fill <<EOF
url=$GIT_URL

EOF
)

  creds_regex=''$'\n''username=([^'$'\n'']+)'$'\n''password=([^'$'\n'']+)'

  if [[ "$CREDS" =~ $creds_regex ]]; then
    GIT_USER="${BASH_REMATCH[1]}"
    GIT_CREDENTIALS="${BASH_REMATCH[2]}"
  fi

  if [ -z "$GIT_CREDENTIALS" ]; then
    echo "Unable to parse GIT_CREDENTIALS from `git credential fill`"
    echo "----- OUTPUT -----"
    echo "$CREDS"
    echo "----- OUTPUT -----"
  fi
}

function git_url() {
  if [ -z "$GIT_URL" ]; then
    GIT_URL=$(grep -e '^\W*url: ' testdata/addons/addons-source.yaml | awk '{print $2}')
  fi
  echo "$GIT_URL"
}

function args() {
  toolkit=""
  dry_run=""
  deploy_kind=0
  gitops_reg=""
  kraan_repo="kraan"
  kraan_name=""
  kraan_reg=""
  kraan_regcred=""
  gitops_regcred=""
  gitops_proxy=""
  git_url
  apply_testdata=0
  kraan_tag="master"
  kraan_loglevel=""
  no_git_auth=0
  helm_action=""
  prometheus=""
  kraan_dev=""
  values_files=""

  arg_list=( "$@" )
  arg_count=${#arg_list[@]}
  arg_index=0
  while (( arg_index < arg_count )); do
    case "${arg_list[${arg_index}]}" in
          "--toolkit") toolkit=1;;
          "--deploy-kind") deploy_kind=1;;
          "--testdata") apply_testdata=1;;
          "--kraan-dev") kraan_dev=1;;
          "--prometheus") (( arg_index+=1 )); prometheus="${arg_list[${arg_index}]}";;
          "--kraan-loglevel") (( arg_index+=1 )); kraan_loglevel="${arg_list[${arg_index}]}";;
          "--kraan-tag") (( arg_index+=1 )); kraan_tag="${arg_list[${arg_index}]}";;
          "--kraan-image-pull-secret") (( arg_index+=1 )); kraan_regcred="${arg_list[${arg_index}]}";;
          "--gitops-image-pull-secret") (( arg_index+=1 )); gitops_regcred="${arg_list[${arg_index}]}";;
          "--gitops-proxy") (( arg_index+=1 )); gitops_proxy="${arg_list[${arg_index}]}";;
          "--kraan-image-repo") (( arg_index+=1 )); kraan_repo="${arg_list[${arg_index}]}";;
          "--kraan-image-reg") (( arg_index+=1 )); kraan_reg="${arg_list[${arg_index}]}";;
          "--kraan-image-name") (( arg_index+=1 )); kraan_name="${arg_list[${arg_index}]}";;
          "--gitops-image-reg") (( arg_index+=1 )); gitops_reg="${arg_list[${arg_index}]}";;
          "--values-files") (( arg_index+=1 )); values_files="${arg_list[${arg_index}]}";;
          "--git-url") (( arg_index+=1 )); GIT_URL="${arg_list[${arg_index}]}";;
          "--git-user") (( arg_index+=1 )); GIT_USER="${arg_list[${arg_index}]}";;
          "--git-token") (( arg_index+=1 )); GIT_CREDENTIALS="${arg_list[${arg_index}]}";;
          "--dry-run") dry_run="--dry-run";;
          "--helm") (( arg_index+=1 )); helm_action="${arg_list[${arg_index}]}";;
          "--no-git-auth") no_git_auth=1;;
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

  if [ $no_git_auth -eq 0 ] ; then
    check_git_credentials
    # If GIT_CREDENTIALS are not set warn the user but set up the cluster without a credentials secret
    if [ -z "${GIT_USER:-}" ] ; then
      echo "GIT_USER is not set to the git user name"
      # usage; exit 1
    fi
    if [ -z "${GIT_CREDENTIALS:-}" ] ; then
        echo "GIT_CREDENTIALS is not set to the git user's password or token"
        # usage; exit 1
    fi
  fi
}

function create_addons_source_yaml {
  local SOURCE="$1"
  local TARGET="$2"
  cp $SOURCE $TARGET
  # If GIT_URL is set, shuffle it into addons-source.yaml in place of the existing URL
  if [ -n "$GIT_URL" ]; then
    sed -r -i "s|^(\W+url: ).*$|\1$GIT_URL|" $TARGET
  fi
  # If GIT_CREDENTIALS is not set, remove the secretRef from addons-source.yaml
  if [[ -z "${GIT_CREDENTIALS}" || $no_git_auth -eq 1 ]] ; then
    sed -r -i "/^\W+secretRef:\W*$/,+1d" $TARGET
  fi
  echo "Applying $TARGET"
  kubectl apply ${dry_run} -f $TARGET
}

function create_git_credentials_secret {
  local SOURCE="$1"
  local TARGET="$2"
  if [[ -z "$GIT_CREDENTIALS" || $no_git_auth -eq 1 ]]; then
    return
  fi
  cp $SOURCE $TARGET
  local base64_user="$(echo -n "${GIT_USER}" | base64 -w 0)"
  local base64_creds="$(echo -n "${GIT_CREDENTIALS}" | base64 -w 0)"
  sed -i -r "s|(^\W+username: ).*$|\1${base64_user}|" $TARGET
  sed -i -r "s|(^\W+password: ).*$|\1${base64_creds}|" $TARGET
  echo "Applying $TARGET"
  kubectl apply ${dry_run} -f $TARGET -n gotk-system
}

function toolkit_refresh() {
  local gitops_repo_arg=""
  local gitops_regcred_arg=""
  local secret_name=""
  if [ -n "${gitops_reg}" ] ; then
    gitops_repo_arg="--registry ${gitops_reg}"
  fi
  if [ -n "${gitops_regcred}" ] ; then
    secret_name="regcred"
    if [ "${gitops_regcred}" != "auto" ] ; then
      secret_name=$(basename "${gitops_regcred}" | cut -f1 -d.)
    fi
    gitops_regcred_arg="--image-pull-secret ${secret_name}"
  fi
  mkdir -p ${work_dir}"/gitops"
  flux install --export --components=source-controller,helm-controller ${gitops_repo_arg} ${gitops_regcred_arg} > "${work_dir}"/gitops/gitops.yaml
  echo "yaml for gitops toolkit is in ${work_dir}/gitops/gitops.yaml"
}

function create_regcred() {
  local namespace="${1}"
  local auto_file="${2}"
  local name_prefix="${3}"
  if [ "${auto_file}" == "auto" ] ; then
    if [ -n "${dry_run}" ] ; then
      return
    fi
    jq -r '{auths: .auths}' ~/.docker/config.json > "${work_dir}"/image_pull_secret.json
    kubectl -n "${namespace}" delete  --ignore-not-found=true secret ${name_prefix}-regcred
    kubectl -n "${namespace}" create secret generic ${name_prefix}-regcred \
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

function install_namespace {
  local ns="${1}"

  set +e
  kubectl get namespace "${ns}" 2>&1
  if [ $? -ne 0 ] ; then
    kubectl create namespace "${ns}"
  else
    echo "namespace ${ns} already exists"
  fi
  set -e
}

function wait_for_pods {
  local ns="${1}"
  for i in {1..30}
  do
    set +e
    kubectl -n ${ns} get pods --field-selector=status.phase!=Running 2>&1 | grep "No resources found in ${ns} namespace" >/dev/null
    if [ $? -eq 0 ] ; then
      set -e
      echo "all pods in namespace ${ns} running"
      kubectl -n ${ns} get pods
      return 0
    fi
    echo "all pods in namespace ${ns} not yet running"
    kubectl -n ${ns} get pods --field-selector=status.phase!=Running
    sleep 5
  done
  return 1
}

function install_prometheus_helm_repo {
  set +e
  helm repo list | grep "^prometheus-community[[:space:]]" >/dev/null
  if [ $? -eq 1 ] ; then
    set -e
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    return
  fi
  set -e
}

function install_prometheus_helm_release {
  local ns="${1}"
  set +e
  helm list -n "${ns}" | grep "^prometheus[[:space:]]" >/dev/null
  if [ $? -eq 1 ] ; then
    set -e
    helm install prometheus prometheus-community/prometheus --namespace "${ns}"
    return
  fi
  set -e
}

function install_prometheus {
  local ns="${1}"
  install_prometheus_helm_repo
  install_namespace "${ns}"
  install_prometheus_helm_release "${ns}"
  set +e
  wait_for_pods "${ns}"
  if [ $? -ne 0 ] ; then
    echo "failed to deploy prometheus"
    exit 1
  fi
  set -e
  echo "running port-forward to prometheus: kubectl port-forward -n "${ns}" prometheus-prometheus-kube-prometheus-prometheus-0  9090"
  kubectl port-forward -n "${ns}" service/prometheus-server  8081:80 &
  echo "You can access prometheus at http:/127.0.0.1:8081"
  # May add Grafana later
  #grafana=$(kubectl -n ${ns} get pods --selector=app.kubernetes.io/name=grafana -o   jsonpath='{.items[*].metadata.name}')
  #echo "running port-forward to grafana: kubectl port-forward -n "${ns}" ${grafana} 3000"
  #kubectl port-forward -n "${ns}" ${grafana} 3000 &
  #echo "You can access grafana at http:/127.0.0.1:3000, grafana user: `kubectl get secret -n "${ns}" prometheus-grafana -o json | jq -r '.data["admin-user"]' | base64 -d`, grafana password: `kubectl get secret -n "${ns}" prometheus-grafana -o json | jq -r '.data["admin-password"]' | base64 -d`"
}

args "$@"

base_dir="$(git rev-parse --show-toplevel)"
work_dir="$(mktemp -d -t kraan-XXXXXX)"

if [ $deploy_kind -gt 0 ] ; then
  "${base_dir}"/scripts/kind.sh
fi

if [ -n "${toolkit}" ] ; then
  toolkit_refresh
  exit 0
fi

if [ -n "${prometheus}" ] ; then
  install_prometheus "${prometheus}"
fi

kubectl apply ${dry_run} -f "${base_dir}"/samples/namespace.yaml

if [ -n "${gitops_regcred}" ] ; then
  create_regcred gotk-system "${gitops_regcred}" gotk
fi
if [ -n "${kraan_regcred}" ] ; then
  create_regcred gotk-system "${kraan_regcred}" kraan
fi

helm_args=""

if [ -n "${gitops_regcred}" ] ; then
  helm_args="${helm_args} --set gotk.sourceController.imagePullSecrets.name=gotk-regcred"
  helm_args="${helm_args} --set gotk.helmController.imagePullSecrets.name=gotk-regcred"
fi
if [ -n "${gitops_reg}" ] ; then
  helm_args="${helm_args} --set gotk.sourceController.image.repository=${gitops_reg}/fluxcd"
  helm_args="${helm_args} --set gotk.helmController.image.repository=${gitops_reg}/fluxcd"
fi
if [ -n "${gitops_proxy}" ] ; then
  if [ "${gitops_proxy}" == "auto" ] ; then
    if [ -n "${HTTPS_PROXY:-}" ] ; then
      gitops_proxy="${HTTPS_PROXY}"
    else
      if [ -n "${https_proxy:-}" ] ; then
        gitops_proxy="${https_proxy}"
      else
        echo "error gitops-proxy set to auto but no https proxy environmental variables set"
        exit 1
      fi
    fi
  fi
  helm_args="${helm_args} --set global.env.httpsProxy=${gitops_proxy}"
  helm_args="${helm_args} --set gotk.sourceController.proxy=true"
  helm_args="${helm_args} --set gotk.helmController.proxy=true"
fi
if [ -n "${kraan_regcred}" ] ; then
  helm_args="${helm_args} --set kraan.kraanController.imagePullSecrets.name=kraan-regcred"
fi
if [ -n "${kraan_reg}" ] ; then
  kraan_reg="${kraan_reg}/"
fi
if [[ -n "${kraan_repo}" || -n "${kraan_reg}" ]] ; then
  helm_args="${helm_args} --set kraan.kraanController.image.repository=${kraan_reg}${kraan_repo}"
fi
if [ -n "${kraan_name}" ] ; then
  helm_args="${helm_args} --set kraan.kraanController.image.name=${kraan_name}"
fi
if [ -n "${kraan_tag}" ] ; then
  helm_args="${helm_args} --set kraan.kraanController.image.tag=${kraan_tag}"
fi
if [ -n "${kraan_loglevel}" ] ; then
  helm_args="${helm_args} --set kraan.kraanController.args.logLevel=${kraan_loglevel}"
fi

if [ -n "${kraan_dev}" ] ; then
  helm_args="${helm_args} --set kraan.kraanController.devmode=false"
fi

if [ -n "${values_files}" ] ; then
  helm_args="${helm_args} --values ${values_files}"
fi

if [ -n "${dry_run}" ] ; then
  helm_args="${helm_args} --dry-run"
fi
if [ -n "${helm_action}" ] ; then
  helm ${helm_action} kraan chart ${helm_args} --namespace gotk-system
fi

if [ $apply_testdata -gt 0 ]; then
  create_git_credentials_secret "${base_dir}/testdata/templates/template-http.yaml" "${work_dir}/kraan-http.yaml"
  create_addons_source_yaml "${base_dir}/testdata/addons/addons-source.yaml" "${work_dir}/addons-source.yaml"
  # Create namespaces for each addon layer
  kubectl apply ${dry_run} -f "${base_dir}"/testdata/namespaces.yaml
  kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/addons-repo.yaml
  kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/addons.yaml
fi

if [ -z "${dry_run}" ] ; then
  rm -rf "${work_dir}"
else
  echo "yaml files in ${work_dir}"
fi
