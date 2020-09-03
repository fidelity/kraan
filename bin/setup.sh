#!/usr/bin/env bash
# Set versions of software required
linter_version=1.30.0
golang_version=1.14.6

function usage()
{
    echo "USAGE: ${0##*/}"
    echo "Install software required for golang project"
}

function args() {
    while [ $# -gt 0 ]
    do
        case "$1" in
            "--help") usage; exit;;
            "-?") usage; exit;;
            *) usage; exit;;
        esac
    done
}

function install_linter() {
    curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b "${PROJECT_BIN_ROOT}" v${linter_version}
}

function install_golang() {
    echo "Installing golang version: ${golang_version}"
    # shellcheck disable=SC2164
    pushd /tmp >/dev/null
    # shellcheck disable=SC1090
    # shellcheck disable=SC2164
    curl -qL -O "https://storage.googleapis.com/golang/go${golang_version}.linux-amd64.tar.gz" &&
      tar xfa go${golang_version}.linux-amd64.tar.gz &&
      rm -rf "${PROJECT_BIN_ROOT}/go" &&
      mv go "${PROJECT_BIN_ROOT}" &&
      source "${SCRIPT_DIR}/env.sh" &&
    popd >/dev/null

    # shellcheck disable=SC2164
    pushd "${GOROOT}/src/go/types" > /dev/null
    echo "Installing gotype linter"
    go build gotype.go
    cp gotype "${GOBIN}"
    # shellcheck disable=SC2164
    popd >/dev/null
}

function install_godocdown() {
    echo "installing godocdown"
    go get github.com/robertkrimen/godocdown/godocdown
}

function make_local() {
    if [ ! -d "${PROJECT_BIN_ROOT}" ] ; then
        echo "Creating directory for ${PROJECT_NAME} software in ${PROJECT_BIN_ROOT}"
        mkdir -p "${PROJECT_BIN_ROOT}"
    fi
    # shellcheck disable=SC1090
    source "${SCRIPT_DIR}/env.sh"
}

SCRIPT_DIR="$(readlink -f "$(dirname "${0}")")"
# shellcheck disable=SC1090
if ! source "${SCRIPT_DIR}/env.sh"; then
    exit 1
fi

args "${@}"

echo "Running setup script to setup software for ${PROJECT_NAME}"

# Remove any legacy installs
rm -rf "${PROJECT_DIR}/bin/local"

make_local

golangci-lint --version 2>&1 | grep $linter_version >/dev/null
ret_code="${?}"
if [[ "${ret_code}" != "0" || ! -e "${PROJECT_BIN_ROOT}/golangci-lint" ]] ; then
    install_linter
    golangci-lint --version 2>&1 | grep $linter_version >/dev/null
    ret_code="${?}"
    if [ "${ret_code}" != "0" ] ; then
        echo "Failed to install linter"
        exit 1
    fi
fi

go version 2>&1 | grep $golang_version >/dev/null
ret_code="${?}"
if [[ "${ret_code}" != "0"  || "${GOROOT}" != "${PROJECT_BIN_ROOT}/go" ]] ; then
    install_golang
    go version 2>&1 | grep $golang_version >/dev/null
    ret_code="${?}"
    if [ "${ret_code}" != "0" ] ; then
        echo "Failed to install golang"
        exit 1
    fi
fi

godocdown >/dev/null 2>&1
ret_code="${?}"
if [[ "${ret_code}" == "127" || "${GOBIN}" != "${PROJECT_BIN_ROOT}" ]] ; then
    install_godocdown
    godocdown >/dev/null 2>&1
    if [ "$?" == "127" ] ; then
        echo "Failed to install godocdown"
        exit 1
    fi
fi

echo "Installing latest version of gitops toolkit cli"
curl -s https://toolkit.fluxcd.io/install.sh | sudo -E bash
