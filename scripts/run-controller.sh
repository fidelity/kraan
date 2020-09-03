#!/bin/bash

set -euo pipefail

function usage()
{
    set +x
    echo "USAGE: ${0##*/} [--debug]"
    echo "Run the Kraan Addon Manager on local machine"
    echo "options:"
   echo "'--debug' for verbose output"
   echo "This script will create a temporary directory and copy the testdata/addons directory to addons-config/testdata subdirectory in"
   echo "the temporary directory. It will then set the environmental variable REPOS_PATH to the temporary directory. This will cause the"
   echo "kraan-controller to process the addons layers using the temporary directory as its root directory when locating the yaml files"
   echo "to process for each addon layer. This enables the kraan-controller to be tested without relying use of the source controller to"
   echo "obtain the yaml files from the git repository."
}

function args() {
  debug=""

  arg_list=( "$@" )
  arg_count=${#arg_list[@]}
  arg_index=0
  while (( arg_index < arg_count )); do
    case "${arg_list[${arg_index}]}" in
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
}

args "$@"
base_dir="$(git rev-parse --show-toplevel)"
work_dir="$(mktemp -d -t kraan-XXXXXX)"
mkdir -p "${work_dir}"/addons-config/testdata
cp -rf "${base_dir}"/testdata/addons "${work_dir}"/addons-config/testdata
export REPOS_PATH="${work_dir}"
echo "Running kraan-controller with REPOS_PATH set to ${REPOS_PATH}"
echo "You may change files in this directory to test kraan-controller"
echo "Edit and then kubectl apply ${work_dir}/addons-config/testdata/addons/addons.yaml to cause kraan-controller to reprocess layers."
echo "if you want change and rerun the kraan-controller you should type..."
echo "export REPOS_PATH=${work_dir}"
echo "kraan-controller"
echo "The temporary directory will not be deleted to allow for this sceanrio so you are responsible for deleting this directory"
kraan-controller
