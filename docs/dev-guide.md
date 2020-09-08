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

    USAGE: setup.sh [--debug] [--dry-run] [--toolkit] [--deploy-kind]
       [--kraan-image-pull-secret auto | <filename>] [--kraan-image-repo <repo-name>]
       [--gitops-image-pull-secret auto | <filename>] [--gitops-image-repo <repo-name>]
    Install the Kraan Addon Manager and gitops source controller to a Kubernetes cluster
    options:
    '--kraan-image-pull-secret' set to 'auto' to generate image pull secrets from ~/.docker/config.json or
                                or supply name of file containing image pull secret defintion to apply.
                                the last element of the filename should also be the secret name, e.g.
                                filename /tmp/regcred.yaml should define a secret called 'regcred'
    '--gitops-image-pull-secret' as above for gitops components
    '--kraan-image-repo' provide image repository to use for Kraan, docker.pkg.github.com/
    '--gitops-image-repo' provide image repository to use for gitops components, defaults to docker.io/fluxcd
    '--deploy-kind' to create a kind cluster and use that to deploy to. Otherwise it will deploy to an existing cluster.
                    The KUBECONFIG environmental variable or ~/.kube/config should be set to a cluster admin user for the
                    cluster you want to use. This cluster must be running API version 16 or greater.
    '--toolkit' to generate GitOps toolkit components
    '--debug' for verbose output
    '--dry-run' to generate yaml but not deploy to cluster. This option will retain temporary work directory
    The environmental variable GIT_USER and GIT_CREDENTIALS must be set to the git user and credentials respectively

## Testing

To test kraan-controller

    scripts/run-controller.sh --help
    USAGE: run-controller.sh [--debug] [--add-secret] [--ignore-test-errors] [--set-git-repo <repo url>] [--set-git-ref <branch name>]
    Run the Kraan Addon Manager on local machine
    options:
    '--debug' for verbose output
    '--ignore-test-errors' update helm releases in testdata to ignore test failures
    '--add-secret' create a secret in kraan namespace for use by helm-operator when accessing git repository containing charts
                the environmental variable GIT_USER and GIT_CREDENTIALS must be set to the git user and credentials respectively
    '--set-git-repo' set the url of the git repo in the testdata helm releases, defaults to 'https://github.com/fidelity/kraan'
    '--set-git-ref' set the git repo branch in the testdata helm releases, defaults to 'master'
    This script will create a temporary directory and copy the testdata/addons directory to addons-config/testdata subdirectory in
    the temporary directory. It will then set the environmental variable REPOS_PATH to the temporary directory. This will cause the
    kraan-controller to process the addons layers using the temporary directory as its root directory when locating the yaml files
    to process for each addon layer. This enables the kraan-controller to be tested without relying use of the source controller to
    obtain the yaml files from the git repository.
