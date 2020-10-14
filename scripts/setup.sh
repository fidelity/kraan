#!/bin/bash

export GIT_URL="${GIT_URL}"
export GIT_USER="${GIT_USER}"
export GIT_CREDENTIALS="${GIT_CREDENTIALS:-$GIT_TOKEN}"

set -euo pipefail

function usage() {
    set +x
    cat <<EOF
USAGE: ${0##*/} [--debug] [--dry-run] [--toolkit] [--deploy-kind] [--no-kraan] [--no-gitops]
       [--kraan-version] [--kraan-image-pull-secret auto | <filename>] [--kraan-image-repo <repo-name>]
       [--gitops-image-pull-secret auto | <filename>] [--gitops-image-repo <repo-name>]
       [--gitops-proxy auto | <proxy-url>] [--git-url <git-repo-url>]
       [--git-user <git_username>] [--git-token <git_token_or_password>]

Install the Kraan Addon Manager and gitops source controller to a Kubernetes cluster

Options:
  '--kraan-image-pull-secret' set to 'auto' to generate image pull secrets from ~/.docker/config.json
                              or supply name of file containing image pull secret defintion to apply.
                              the last element of the filename should also be the secret name, e.g.
                              filename /tmp/regcred.yaml should define a secret called 'regcred'
  '--gitops-image-pull-secret' as above for gitops components
  '--kraan-image-repo' provide image repository to use for Kraan, defaults to docker.pkg.github.com/
  '--kraan-version' the version of the kraan image to use.
  '--gitops-image-repo' provide image repository to use for gitops components, defaults to docker.io/fluxcd
  '--gitops-proxy' set to 'auto' to generate proxy setting for source controller using value of HTTPS_PROXY 
                   environment variable or supply the proxy url to use.
  '--deploy-kind' create a new kind cluster and deploy to it. Otherwise the script will deploy to an existing 
                  cluster. The KUBECONFIG environmental variable or ~/.kube/config should be set to a cluster 
                  admin user for the cluster you want to use. This cluster must be running API version 16 or 
                  greater.
  '--no-kraan' do not deploy the Kraan runtime container to the target cluster. Useful when running controller out of cluster.
  '--no-gitops' do not deploy the gitops system components to the target cluster. Useful if components are already installed"
  '--no-testdata' do not deploy addons layers and source controller custom resources to the target cluster.
  '--git-user' set (or override) the GIT_USER environment variables.
  '--git-token' set (or override) the GIT_CREDENTIALS environment variables.
  '--git-url' set the URL for the git repository from which Kraan should pull AddonsLayer configs.
  '--no-git-auth' to indicate that no authorization is required to access git repository'
  '--toolkit' to generate GitOps toolkit components.
  '--debug' for verbose output.
  '--dry-run' to generate yaml but not deploy to cluster. This option will retain temporary work directory.
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
  gitops_repo=""
  kraan_repo=""
  kraan_regcred=""
  gitops_regcred=""
  gitops_proxy=""
  git_url
  deploy_kraan=1
  deploy_gitops=1
  apply_testdata=1
  kraan_version="latest"
  no_git_auth=0

  arg_list=( "$@" )
  arg_count=${#arg_list[@]}
  arg_index=0
  while (( arg_index < arg_count )); do
    case "${arg_list[${arg_index}]}" in
          "--toolkit") toolkit=1;;
          "--deploy-kind") deploy_kind=1;;
          "--no-kraan") deploy_kraan=0;;
          "--no-gitops") deploy_gitops=0;;
          "--no-testdata") apply_testdata=0;;
          "--kraan-version") (( arg_index+=1 )); kraan_version="${arg_list[${arg_index}]}";;
          "--kraan-image-pull-secret") (( arg_index+=1 )); kraan_regcred="${arg_list[${arg_index}]}";;
          "--gitops-image-pull-secret") (( arg_index+=1 )); gitops_regcred="${arg_list[${arg_index}]}";toolkit=1;;
          "--gitops-proxy") (( arg_index+=1 )); gitops_proxy="${arg_list[${arg_index}]}";toolkit=1;;
          "--kraan-image-repo") (( arg_index+=1 )); kraan_repo="${arg_list[${arg_index}]}";;
          "--gitops-image-repo") (( arg_index+=1 )); gitops_repo="${arg_list[${arg_index}]}";toolkit=1;;
          "--git-url") (( arg_index+=1 )); GIT_URL="${arg_list[${arg_index}]}";;
          "--git-user") (( arg_index+=1 )); GIT_USER="${arg_list[${arg_index}]}";;
          "--git-token") (( arg_index+=1 )); GIT_CREDENTIALS="${arg_list[${arg_index}]}";;
          "--dry-run") dry_run="--dry-run";;
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

  check_git_credentials
  if [ $no_git_auth -eq 0 ] ; then
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
  gotk install --export --components=source-controller,helm-controller ${gitops_repo_arg} ${gitops_regcred_arg} > "${work_dir}"/gitops/gitops.yaml
  if [ -n "${dry_run}" ] ; then
    echo "yaml for gitops toolkit is in ${work_dir}/gitops/gitops.yaml"
  fi
  if [ -n "${gitops_proxy}" ] ; then
    local gitops_proxy_url="${gitops_proxy}"
  if [ "${gitops_proxy}" == "auto" ] ; then
    gitops_proxy_url="${HTTPS_PROXY}"
  fi
    cp "${work_dir}"/gitops/gitops.yaml "${work_dir}"/gitops/gitops-orignal.yaml
    awk '/env:/{ print;print "        - name: HTTPS_PROXY\n          value: gitops_proxy_url\n        - name: NO_PROXY\n          value: 10.0.0.0/8,172.0.0.0/8";next}1' \
        "${work_dir}"/gitops/gitops-orignal.yaml > "${work_dir}"/gitops/gitops.yaml
    sed -i "s#gitops_proxy_url#${gitops_proxy_url}#" "${work_dir}"/gitops/gitops.yaml
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
  if [ -n "${kraan_regcred}" ] ; then
    create_regcred gotk-system "${kraan_regcred}"
  fi
  cp -rf "${base_dir}"/testdata/addons/kraan/manager "${work_dir}"
  if [ -n "${kraan_repo}" ] ; then
    sed -i "s#image\:\ docker.pkg.github.com/fidelity/kraan#image\:\ ${kraan_repo}#" "${work_dir}"/manager/deployment.yaml
  fi
  if [ -n "${kraan_version}" ] ; then
    sed -i "s#\:latest#\:${kraan_version}#" "${work_dir}"/manager/deployment.yaml
  fi
  if [ -n "${kraan_regcred}" ] ; then
    local secret_name="regcred"
    if [ "${kraan_regcred}" != "auto" ] ; then
      secret_name=$(basename "${kraan_regcred}" | cut -f1 -d.)
    fi
    cp "${work_dir}"/manager/deployment.yaml "${work_dir}"/deployment-orignal.yaml
    awk '/containers:/{ print "      imagePullSecrets:\n      - name: regcred"}1'  "${work_dir}"/deployment-orignal.yaml > "${work_dir}"/manager/deployment.yaml
    sed -i "s#\:\ regcred#\:\ ${secret_name}#" "${work_dir}"/manager/deployment.yaml
  fi
  if [ $deploy_kraan -gt 0 ]; then
    echo "Deploying Kraan Manager"
    kubectl apply ${dry_run} -f "${work_dir}"/manager
  fi
}

args "$@"

base_dir="$(git rev-parse --show-toplevel)"
work_dir="$(mktemp -d -t kraan-XXXXXX)"

if [ $deploy_kind -gt 0 ] ; then
  "${base_dir}"/scripts/kind.sh
fi

if [ $deploy_gitops -gt 0 ] ; then 
  cp -rf "${base_dir}"/testdata/addons/gitops "${work_dir}"

  if [ -n "${toolkit}" ] ; then
    toolkit_refresh
  fi

  if [ -n "${dry_run}" ] ; then
    echo "yaml for gitops toolkit is in ${work_dir}/gitops/gitops.yaml"
  fi

  kubectl apply ${dry_run} -f "${work_dir}"/gitops/gitops.yaml

  create_git_credentials_secret "${base_dir}/testdata/templates/template-http.yaml" "${work_dir}/kraan-http.yaml"

  if [ -n "${gitops_regcred}" ] ; then
    create_regcred gotk-system "${gitops_regcred}"
  fi
fi

kubectl apply ${dry_run} -k "${base_dir}"/config/crd
kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/kraan/rbac/serviceaccount.yaml
kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/kraan/rbac/leader_election_role.yaml
kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/kraan/rbac/leader_election_role_binding.yaml
kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/kraan/rbac/role.yaml
kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/kraan/rbac/role_binding.yaml

deploy_kraan_mgr

if [ $apply_testdata -gt 0 ]; then
  create_addons_source_yaml "${base_dir}/testdata/addons/addons-source.yaml" "${work_dir}/addons-source.yaml"
  # Create namespaces for each addon layer
  kubectl apply ${dry_run} -f "${base_dir}"/testdata/namespaces.yaml
  kubectl apply ${dry_run} -f "${base_dir}"/testdata/addons/addons.yaml
fi

if [ -z "${dry_run}" ] ; then
  rm -rf "${work_dir}"
fi
