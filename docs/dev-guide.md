# Developsrs Guide

This readme contains guidance for developers. covering building the kraan-controller, setting up a development environment and testing the kraan-controller.

## Setup

clone into $GOPATH/src/github.com/fidelity/kraan:

    mkdir -p $GOPATH/src/github.com/fidelity/kraan
    cd $GOPATH/src/github.com/fidelity/
    git clone git@github.com:fidelity/kraan.git
    cd kraan

This project requires the following software:

    golangci-lint --version = 1.30.0
    golang version = 1.14.6
    godocdown version = head

You can install these in the project bin directory using the 'setup.sh' script:

    . bin/env.sh
    setup.sh

The setup.sh script can safely be run at any time. It installs the required software in the $GOPATH/bin/`<project-org>`/`<project-name>` directory, where `<project-org>` is the git organisation name and `<project-name>` is the git respository name, i.e. `fidelity`/`kraan`

## Development

The Makefile in the project's top level directory will compile, build and test all components.

    make

To build docker image type:

    make build

If changes are made to go source imports you may need to perform a go mod vendor, type:

    make gomod-update

## Setup Test Environment

A shell script is provided to deploy the artifacts necessary to test the kraan-controller to a kubernetes cluster.

    scripts/setup.sh --help

    USAGE: setup.sh [--debug] [--dry-run] [--toolkit] [--deploy-kind] [--no-kraan]
       [--helm-operator-namespace <namespace>]
       [--kraan-image-pull-secret auto | <filename>] [--kraan-image-repo <repo-name>]
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
    '--install-helm-operator' deploy the Helm Operator to kraan namespace.
    '--kraan-image-repo' provide image repository to use for Kraan, docker.pkg.github.com/
    '--gitops-image-repo' provide image repository to use for gitops components, defaults to docker.io/fluxcd
    '--gitops-proxy' set to 'auto' to generate proxy setting for source controller using value of HTTPS_PROXY 
                    environment variable or supply the proxy url to use.
    '--deploy-kind' create a new kind cluster and deploy to it. Otherwise the script will deploy to an existing 
                    cluster. The KUBECONFIG environmental variable or ~/.kube/config should be set to a cluster 
                    admin user for the cluster you want to use. This cluster must be running API version 16 or 
                    greater.
    '--no-kraan' do not deploy the Kraan runtime container to the target cluster.
    '--no-testdata' do not deploy addons layers and source controller custom resources to the target cluster.
    '--git-user' set (or override) the GIT_USER environment variables.
    '--git-token' set (or override) the GIT_CREDENTIALS environment variables.
    '--git-url' set the URL for the git repository from which Kraan should pull AddonsLayer configs.
    '--toolkit' to generate GitOps toolkit components.
    '--debug' for verbose output.
    '--dry-run' to generate yaml but not deploy to cluster. This option will retain temporary work directory.

## Testing

To test kraan-controller

    scripts/run-controller.sh --help
    USAGE: run-controller.sh [--debug]
    Run the Kraan Addon Manager on local machine
    options:
    '--debug' for verbose output
    This script will create a temporary directory and copy the addons.yaml and addons-source.yam files from testdata/addons to
    the temporary directory. It will then set the environmental variable DATA_PATH to the temporary directory. This will cause the
    kraan-controller to process the addons layers using the temporary directory as its root directory when storing files it retrieves
    from this git repository's testdata/addons directory using the source controller.


The kraan-controller will reprocess all AddonsLayers perioidically. This period defaults to 30 seconds but can be set using a command line argument.

    kraan-controller -sync-period=1m

The reprocessing period can also be set to a period in seconds using the 's' suffix, i.e. 20s.

The SC_TIMEOUT environmental variable can be used to set the timeout period for retrieving data from the source controller, default is 15 seconds.

    export SC_TIMEOUT=30s

The SC_HOST environmental variable can be used to set the host component of the source controller's artifact url. This is useful when running the
kraan-controller out of cluster to enable it to access the source controller via a local address using kubectl port-forward, i.e.

    kubectl -n gitops-system port-forward svc/source-controller 8090:80 &
	export SC_HOST=localhost:8090
