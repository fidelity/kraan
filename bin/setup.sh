#!/usr/bin/env bash
# Set versions of software required
linter_version=1.30.0
kubebuilder_version=2.3.1
mockgen_version=v1.4.4
helm_version=v3.3.4
kind_version=v0.9.0

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
    TARGET=$(go env GOPATH)
    curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b "${TARGET}/bin" v${linter_version}
}

function install_kubebuilder() {
    os=$(go env GOOS)
    arch=$(go env GOARCH)

    # download kubebuilder and extract it to tmp
    curl -L https://go.kubebuilder.io/dl/${kubebuilder_version}/${os}/${arch} | tar -xz -C /tmp/

    # move to a long-term location and put it on your path
    # (you'll need to set the KUBEBUILDER_ASSETS env var if you put it somewhere else)
    $sudo mv /tmp/kubebuilder_${kubebuilder_version}_${os}_${arch} /usr/local/kubebuilder
    echo "add the following to your bash profile"
    echo "export PATH=\$PATH:/usr/local/kubebuilder/bin"
}

function install_helm() {
    curl -L https://get.helm.sh/helm-${helm_version}-linux-amd64.tar.gz | tar -xz -C /tmp/
    $sudo mv /tmp/linux-amd64/helm /usr/local/bin
}

function install_kind() {
    curl -Lo ./kind https://kind.sigs.k8s.io/dl/${kind_version}/kind-linux-amd64
    chmod 755 kind
    $sudo mv kind /usr/local/bin
}

args "${@}"

sudo -E env >/dev/null 2>&1
if [ $? -eq 0 ]; then
    sudo="sudo -E"
fi

echo "Running setup script to setup software"

golangci-lint --version 2>&1 | grep $linter_version >/dev/null
ret_code="${?}"
if [[ "${ret_code}" != "0" ]] ; then
    echo "installing linter version: ${linter_version}"
    install_linter
    golangci-lint --version 2>&1 | grep $linter_version >/dev/null
    ret_code="${?}"
    if [ "${ret_code}" != "0" ] ; then
        echo "Failed to install linter"
        return
    fi
else
    echo "linter version: `golangci-lint --version`"
fi

export PATH=$PATH:/usr/local/kubebuilder/bin
kubebuilder version 2>&1 | grep ${kubebuilder_version} >/dev/null
ret_code="${?}"
if [[ "${ret_code}" != "0" ]] ; then
    echo "installing kubebuilder version: ${kubebuilder_version}"
    install_kubebuilder
    kubebuilder version 2>&1 | grep ${kubebuilder_version} >/dev/null
    ret_code="${?}"
    if [ "${ret_code}" != "0" ] ; then
        echo "Failed to install kubebuilder"
        return
    fi
else
   echo "kubebuilder version: `kubebuilder version`"     
fi

mockgen -version 2>&1 | grep ${mockgen_version} >/dev/null
ret_code="${?}"
if [[ "${ret_code}" != "0" ]] ; then
    echo "installing mockgen version: ${mockgen_version}"
    GO111MODULE=on go get github.com/golang/mock/mockgen@${mockgen_version}
    mockgen -version 2>&1 | grep ${mockgen_version} >/dev/null
    ret_code="${?}"
    if [ "${ret_code}" != "0" ] ; then
        echo "Failed to install helm"
        return
    fi
else
    echo "mockgen version: `mockgen -version`"
fi

flux --version >/dev/null 2>&1 
ret_code="${?}"
if [[ "${ret_code}" != "0" ]] ; then
    echo "Installing latest version of flux cli"
    curl -s https://toolkit.fluxcd.io/install.sh | $sudo bash
    flux --version >/dev/null 2>&1 
    ret_code="${?}"
    if [ "${ret_code}" != "0" ] ; then
        echo "Failed to install flux"
        return
    fi
else
    echo "flux version: `flux --version`"
fi

helm version 2>&1 | grep ${helm_version} >/dev/null
ret_code="${?}"
if [[ "${ret_code}" != "0" ]] ; then
    echo "installing helm version: ${helm_version}"
    install_helm
    helm version 2>&1 | grep ${helm_version} >/dev/null
    ret_code="${?}"
    if [ "${ret_code}" != "0" ] ; then
        echo "Failed to install helm"
        return
    fi
else
    echo "helm version: `helm version`"
fi

kind version 2>&1 | grep ${kind_version} >/dev/null
ret_code="${?}"
if [[ "${ret_code}" != "0" ]] ; then
    echo "installing kind version: ${kind_version}"
    install_kind
    kind version 2>&1 | grep ${kind_version} >/dev/null
    ret_code="${?}"
    if [ "${ret_code}" != "0" ] ; then
        echo "Failed to install kind"
        return
    fi
else
    echo "kind version: `kind version`"
fi