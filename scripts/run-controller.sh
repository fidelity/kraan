#!/bin/bash

set -euo pipefail

function usage()
{
    set +x
    echo "USAGE: ${0##*/} [--debug] [--add-secret] [--ignore-test-errors] [--set-git-repo <repo url>] [--set-git-ref <branch name>]"
    echo "Run the Kraan Addon Manager on local machine"
    echo "options:"
   echo "'--debug' for verbose output"
   echo "'--ignore-test-errors' update helm releases in testdata to ignore test failures"
   echo "'--add-secret' create a secret in kraan namespace for use by helm-operator when accessing git repository containing charts"
   echo "               the environmental variable GIT_USER and GIT_CREDENTIALS must be set to the git user and credentials respectively"
   echo "'--set-git-repo' set the url of the git repo in the testdata helm releases, defaults to 'https://github.com/fidelity/kraan'"
   echo "'--set-git-ref' set the git repo branch in the testdata helm releases, defaults to 'master'"
   echo "This script will create a temporary directory and copy the testdata/addons directory to addons-config/testdata subdirectory in"
   echo "the temporary directory. It will then set the environmental variable DATA_PATH to the temporary directory. This will cause the"
   echo "kraan-controller to process the addons layers using the temporary directory as its root directory when locating the yaml files"
   echo "to process for each addon layer. This enables the kraan-controller to be tested without relying use of the source controller to"
   echo "obtain the yaml files from the git repository."
}

function args() {
  debug=""
  add_secret=""
  ignore_test_errors=""
  set_git_ref=""
  set_git_repo=""

  arg_list=( "$@" )
  arg_count=${#arg_list[@]}
  arg_index=0
  while (( arg_index < arg_count )); do
    case "${arg_list[${arg_index}]}" in
           "--add-secret") add_secret=1;;
           "--ignore-test-errors") ignore_test_errors=1;;
           "--set-git-repo") (( arg_index+=1 )); set_git_repo="${arg_list[${arg_index}]}";;
           "--set-git-ref") (( arg_index+=1 )); set_git_ref="${arg_list[${arg_index}]}";;
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
  if [ -n "${add_secret}" ] ; then
    if [ -z "${GIT_USER:-}" ] ; then 
      echo "GIT_USER must be set to the git user name"
      usage; exit 1
    fi
    if [ -z "${GIT_CREDENTIALS:-}" ] ; then
        echo "GIT_CREDENTIALS must be set to the git user's password or token"
        usage; exit 1
    fi
  fi
}


function create_secrets {
  local name_space="${1}"
  local base64_user="$(echo -n "${GIT_USER}" | base64 -w 0)"
  local base64_creds="$(echo -n "${GIT_CREDENTIALS}" | base64 -w 0)"
  sed s/GIT_USER/${base64_user}/ "${base_dir}/"testdata/templates/template-http.yaml | \
  sed s/GIT_CREDENTIALS/${base64_creds}/ > "${work_dir}"/hr-http.yaml
  kubectl apply -f "${work_dir}"/hr-http.yaml -n ${name_space}
}

function updateHRs() {
  if [ -n "${add_secret}" ] ; then
    echo "adding secretRef to helm releases access git repo containing test charts"
    for hr_file in `find $DATA_PATH -name microservice*.yaml`
    do
      save_file="${work_dir}"/`basename ${hr_file}`
      cp "${hr_file}" "${save_file}"
          awk '/ref: master/{ print "    secretRef:\n      name: kraan-http"}1' \
              "${save_file}" > "${hr_file}"
      name_space=`grep "namespace:" ${hr_file} | awk -F: '{print $2}'`
      create_secrets $name_space
      rm "${save_file}"
    done
    create_secrets kraan
  fi
  if [ -n "${ignore_test_errors}" ] ; then
    echo "setting ignore failed tests due to issue with podinfo tests"
    for hr_file in `find $DATA_PATH -name microservice*.yaml`
    do
      sed -i s/ignoreFailures\:\ false/ignoreFailures\:\ true/ "${hr_file}"
    done
  fi
  if [ -n "${set_git_ref}" ] ; then
    echo "setting git repo branch to obtain test charts from to: ${set_git_ref}"
    for hr_file in `find $DATA_PATH -name microservice*.yaml`
    do
      sed -i s/ref\:\ master/ref\:\ ${set_git_ref}/ "${hr_file}"
    done
  fi
  if [ -n "${set_git_repo}" ] ; then
    echo "setting git repo to obtain test charts from to: ${set_git_repo}"
    for hr_file in `find $DATA_PATH -name microservice*.yaml`
    do
      sed -i s!git\:\ https\://github.com/fidelity/kraan!git\:\ ${set_git_repo}! "${hr_file}"
    done
  fi
}

args "$@"
base_dir="$(git rev-parse --show-toplevel)"
work_dir="$(mktemp -d -t kraan-XXXXXX)"
mkdir -p "${work_dir}"/testdata/addons
cp -rf "${base_dir}"/testdata/addons/addons*.yaml "${work_dir}"/testdata/addons
export DATA_PATH="${work_dir}"
updateHRs
echo "Running kraan-controller with DATA_PATH set to ${DATA_PATH}"
echo "You may change files in ${work_dir}/testdata/addons to test kraan-controller"
echo "Edit and then kubectl apply ${work_dir}/testdata/addons/addons.yaml to cause kraan-controller to reprocess layers."
echo "Edit and then kubectl apply ${work_dir}/testdata/addons/addons-source.yaml to cause kraan-controller to reprocess source controller data."
echo "if you want change and rerun the kraan-controller you should type..."
echo "export DATA_PATH=${work_dir}"
echo "kubectl -n gitops-system port-forward svc/source-controller 8090:80 &"
echo "export SC_HOST=localhost:8090"
echo "kraan-controller"
echo "In order to allow for this scenario thw temporary directory will not be deleted so you are responsible for deleting this directory"
echo ""
read -p "Pausing to allow user to make manual changes to testdata in ${work_dir}/testdata/addons, press enter to continue"
kubectl -n gitops-system port-forward svc/source-controller 8090:80 &
export SC_HOST=localhost:8090
kraan-controller
