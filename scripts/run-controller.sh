#!/bin/bash

set -euo pipefail

function usage()
{
    set +x
    echo "USAGE: ${0##*/} [--log-level N] [--debug]"
    echo "Run the Kraan Addon Manager on local machine"
    echo "options:"
   echo "'--log-level' N, where N is 1 for debug message or 2 for trace level debugging"
   echo "'--debug' for verbose output"
   echo "This script will create a temporary directory and copy the addons.yaml and addons-source.yam files from testdata/addons to"
   echo "the temporary directory. It will then set the environmental variable DATA_PATH to the temporary directory. This will cause the"
   echo "kraan-controller to process the addons layers using the temporary directory as its root directory when storing files it retrieves"
   echo "from this git repository's testdata/addons directory using the source controller."
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
           "--log-level") (( arg_index+=1 )); log_level="-zap-log-level=${arg_list[${arg_index}]}";;
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
work_dir="$(mktemp -d -t kraan-XXXXXX)"
mkdir -p "${work_dir}"/testdata/addons
cp -rf "${base_dir}"/testdata/addons/addons*.yaml "${work_dir}"/testdata/addons
export DATA_PATH="${work_dir}"
echo "Running kraan-controller with DATA_PATH set to ${DATA_PATH}"
echo "You may change files in ${work_dir}/testdata/addons to test kraan-controller"
echo "Edit and then kubectl apply ${work_dir}/testdata/addons/addons.yaml to cause kraan-controller to reprocess layers."
echo "Edit and then kubectl apply ${work_dir}/testdata/addons/addons-source.yaml to cause kraan-controller to reprocess source controller data."
echo "In order to allow for this scenario the temporary directory will not be deleted so you are responsible for deleting this directory"
echo ""
echo "if you want change and rerun the kraan-controller you should type..."
echo "kubectl -n gotk-system port-forward svc/source-controller 8090:80 &"
echo "export DATA_PATH=${work_dir}"
echo "export SC_HOST=localhost:8090"
echo "kraan-controller -zap-encoder=json ${log_level} 2>&1 | tee ${DATA_PATH}/kraan.log | grep "^{" | jq -r"
read -p "Pausing to allow user to make manual changes to testdata in ${work_dir}/testdata/addons, press enter to continue"

kubectl -n gotk-system port-forward svc/source-controller 8090:80 &
export SC_HOST=localhost:8090
kraan-controller -zap-encoder=json ${log_level} 2>&1 | tee ${work_dir}/kraan.log | grep "^{" | jq -r
