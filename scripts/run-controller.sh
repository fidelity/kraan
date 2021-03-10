#!/bin/bash

set -euo pipefail

function usage()
{
    set +x
    echo "USAGE: ${0##*/} [--log-level N] [--debug]"
    echo "Run the Kraan Addon Manager on local machine"
    echo "options:"
   echo "'--log-level' N, where N is 1 for debug message and 2 or higher for trace level debugging"
   echo "'--debug' for verbose output"
   echo "This script will create a temporary directory call /tmp/kraan-local-exec which the kraan-controller"
   echo "will use as its root directory when storing files it retrieves from this git repository"
   echo "It assumes that the source-controller service is running in gotk-system namespace."
   echo "Also, Kraan Controller namespace is set to gotk-system too using RUNTIME_NAMESPACE environmental variable."
}

function args() {
  arg_list=( "$@" )
  arg_count=${#arg_list[@]}
  arg_index=0
  log_level=""
  while (( arg_index < arg_count )); do
    case "${arg_list[${arg_index}]}" in
           "--debug") set -x;;
               "-h") usage; exit;;
           "--help") usage; exit;;
               "-?") usage; exit;;
           "--log-level") (( arg_index+=1 )); log_level="--zap-log-level=${arg_list[${arg_index}]}";;
        *) if [ "${arg_list[${arg_index}]:0:2}" == "--" ];then
               echo "invalid argument: ${arg_list[${arg_index}]}"
               usage; exit
           fi;
           break;;
    esac
    (( arg_index+=1 ))
  done
}

args "$@"
base_dir="$(git rev-parse --show-toplevel)"
work_dir="${TMPDIR:-/tmp}/kraan-local-exec"
mkdir -p "${work_dir}"
export DATA_PATH="${work_dir}"
echo "Running kraan-controller with DATA_PATH set to ${DATA_PATH}"
echo ""
echo "if you want change and rerun the kraan-controller you should type..."
echo "kubectl -n gotk-system port-forward svc/source-controller 8090:80 &"
echo "export DATA_PATH=${work_dir}"
echo "export SC_HOST=localhost:8090"
echo "kraan-controller -zap-encoder=json ${log_level} 2>&1 | tee ${DATA_PATH}/kraan.log | grep "^{" | jq -r"
read -p "press enter to continue"

kubectl -n gotk-system port-forward svc/source-controller 8090:80 &
export SC_HOST=localhost:8090
export RUNTIME_NAMESPACE=gotk-system
kraan-controller -zap-encoder=json ${log_level} 2>&1 | tee ${work_dir}/kraan.log | grep "^{" | jq -r
