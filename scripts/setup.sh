#!/bin/bash

set -euo pipefail

function usage()
{
    set +x
    echo "USAGE: ${0##*/} [--debug] [--dry-run] [--toolkit]"
	echo "Install the Kraan Addon Manager and gitops source controller to a Kubernetes cluster"
	echo "options:"
  echo "'--debug' for verbose output"
  echo "'--toolkit' to generate GitOps toolkit components"
  echo "'--dry-run' to generate yaml but not deploy to cluster. This option will retain temporary work directory"
  echo "The environmental variable GIT_USER and GIT_CREDENTIALS must be set to the git user and credentials respectively"
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
  sed s/GIT_USER/${base64_user}/ "${base_dir}"testdata/templates/template-http.yaml | \
  sed s/GIT_CREDENTIALS/${base64_creds}/ > "${work_dir}"/kraan-http.yaml
}

args "$@"

base_dir="$(git rev-parse --show-toplevel)"
work_dir="$(mktemp -d -t kraan-XXXXXX)"

create_secrets

if [ -n "${dry_run}" ] ; then
  echo "Yaml files are in ${work_dir}"
  exit 0
fi


rm -rf "${work_dir}"
