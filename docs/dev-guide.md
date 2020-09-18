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

    scripts/setup.sh --help
    https://github.com/fidelity/kraan.git
    USAGE: setup.sh [--debug] [--dry-run] [--toolkit] [--deploy-kind] [--no-kraan]
       [--helm-operator-namespace <namespace>] [--install-helm-operator]
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
    '--install-helm-operator' deploy the Helm Operator.
    '--helm-operator-namespace' set the namespace to install helm-operator in, defaults to 'helm-operator'.
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

When you run the 'run-controller.sh' script it will copy some files the a temporary ditrectory, emmit text as shown below and then prompt you as follow:

    Running kraan-controller with DATA_PATH set to /tmp/kraan-XJQUWB
    You may change files in /tmp/kraan-XJQUWB/testdata/addons to test kraan-controller
    Edit and then kubectl apply /tmp/kraan-XJQUWB/testdata/addons/addons.yaml to cause kraan-controller to reprocess layers.
    Edit and then kubectl apply /tmp/kraan-XJQUWB/testdata/addons/addons-source.yaml to cause kraan-controller to reprocess source controller data.
    In order to allow for this scenario the temporary directory will not be deleted so you are responsible for deleting this directory

    if you want change and rerun the kraan-controller you should type...
    export DATA_PATH=/tmp/kraan-XJQUWB
    kubectl -n gitops-system port-forward svc/source-controller 8090:80 &
    export SC_HOST=localhost:8090
    kraan-controller
    Pausing to allow user to make manual changes to testdata in /tmp/kraan-XJQUWB/testdata/addons, press enter to continue

The kraan-controller will reprocess all AddonsLayers perioidically. This period defaults to 30 seconds but can be set using a command line argument.

    kraan-controller -sync-period=1m

The reprocessing period can also be set to a period in seconds using the 's' suffix, i.e. 20s.

The SC_TIMEOUT environmental variable can be used to set the timeout period for retrieving data from the source controller, default is 15 seconds.

    export SC_TIMEOUT=30s

The SC_HOST environmental variable can be used to set the host component of the source controller's artifact url. This is useful when running the
kraan-controller out of cluster to enable it to access the source controller via a local address using kubectl port-forward, i.e.

    kubectl -n gitops-system port-forward svc/source-controller 8090:80 &
	export SC_HOST=localhost:8090

If you elected to use the '--no-testdata' option when setting up the cluster then you will need to apply these files. You can do this by applying
'.testdata/addons/addons-source.yaml' and '.testdata/addons/addons.yaml' to deploy the source controller custom resource and AddonsLayers custom
resources respectively. This will cause the kraan-controller to operate on the testdata in the './testdata' directory of this repository using the
'master' branch. If you want to test against other branches use the copy of these files the 'scripts/run-controller.sh' creates. Edit then apply
those files.

If you want to use the kraan-controller to deploy items defined in your own repository edit the 'addons-source.yaml' file in the temporary directory
to reference the repository and branch contaiining your addons definitions and apply it to the cluster. Then edit the '.testdata/addons/addons.yaml'
file to define the addons layers and apply that.
