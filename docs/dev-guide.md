# Developers Guide

This readme contains guidance for developers. covering building the kraan-controller, setting up a development environment and testing the kraan-controller.

## Setup

clone [[Kraan repository]](https://github.com/fidelity/kraan) or fork it and clone your fork.

This project requires the following software:

    golangci-lint version = 1.30.0
    golang version >= 1.14.6
    mockgen = v1.4.4
    kubebuilder = v2.3.1
    gotk = latest
    helm - v3.3.4

You can install [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder), [golangci-lint](https://github.com/golangci/golangci-lint), [mockgen](https://github.com/golang/mock), [helm](https://helm.sh/) and [gotk](https://toolkit.fluxcd.io/) using the 'setup.sh' script:

    source bin/setup.sh

## Build

The Makefile in the project's top level directory will compile, build and test all components.

    make

If changes are made to go source imports you may need to perform a go mod vendor, type:

    make gomod-update

Alternatively you can run make in a docker container.

    make check

To build docker image type:

    export REPO=<name of docker repository>
    export VERSION=<version>
    make build

Then to deploy the docker container:

    docker login <docker repository>
    make docker-push

If the `VERSION` is not set `master` is used. The REPO will default to `docker.pkg.github.com/fidelity/kraan` which only users with privileges on the `fidelity` organization will be able to push to. However if you fork the Kraan repository then the default will be `docker.pkg.github.com/<org/user you forked to>/kraan`.

## Releases
The offical releases use docker hub reporistory: kraan/kraan-controller. The 'master' tag will be contain a version of kraan built from the master branch. Release versions will be deployed to this repository for public use.

Helm Charts are deployed to github pages so can be accessed via repo: kraan https://fidelity.github.io/kraan/

    helm repo add kraan https://fidelity.github.io/kraan/
    helm search repo --regexp kraan --versions

## Creating a Release
To create a release set `VERSION` to the release version and `REPO` to 'kraan' then build and push the image:

    export VERSION=vx.y.z
    export REPO=kraan
    make clean-build
    make build
    make docker-push
    make release

The 'release' target will update the tag in the values file, package the chart and deploy it to gh-pages.

Finally use githuweb interface to create a tag.

## Deployment Guide

A shell script is provided to deploy the artifacts necessary to test the kraan-controller to a kubernetes cluster. It does this using a helm client to install a helm chart containing the Kraan Controller and GitOps Toolkit (GOTK) components it uses.

    scripts/setup.sh --help

    https://github.com/fidelity/kraan.git
    USAGE: setup.sh [--debug] [--dry-run] [--toolkit] [--deploy-kind] [--testdata]
        [--kraan-image-reg <registry name>] [--kraan-image-repo <repo-name>] [--kraan-image-tag] 
        [--kraan-image-pull-secret auto | <filename>] [--gitops-image-pull-secret auto | <filename>]
        [--gitops-image-reg <repo-name>]
        [--gitops-proxy auto | <proxy-url>] [--git-url <git-repo-url>] [--no-git-auth]
        [--git-user <git_username>] [--git-token <git_token_or_password>]

    Install the Kraan Addon Manager and gitops source controller to a Kubernetes cluster

    Options:
    '--kraan-image-pull-secret' set to 'auto' to generate image pull secrets from ~/.docker/config.json
                                or supply name of file containing image pull secret defintion to apply.
                                The secret should be called 'kraan-regcred'.
    '--kraan-image-reg'   provide image registry to use for Kraan, defaults to empty for docker hub
    '--kraan-image-repo'  provide image repository prefix to use for Kraan, defaults to 'kraan' for docker hub org.
    '--kraan-tag'         the tag of the kraan image to use.

    '--gitops-image-reg'  provide image registry to use for gitops toolkit components, defaults to 'ghcr.io'.
    '--gitops-image-pull-secret' set to 'auto' to generate image pull secrets from ~/.docker/config.json
                                or supply name of file containing image pull secret defintion to apply.
                                The secret should be called 'gotk-regcred'.
    '--gitops-proxy'    set to 'auto' to generate proxy setting for gotk components using value of HTTPS_PROXY 
                        environment variable or supply the proxy url to use.

    '--deploy-kind' create a new kind cluster and deploy to it. Otherwise the script will deploy to an existing 
                    cluster. The KUBECONFIG environmental variable or ~/.kube/config should be set to a cluster 
                    admin user for the cluster you want to use. This cluster must be running API version 16 or 
                    greater.
    '--testdata'    deploy testdata comprising addons layers and source controller custom resources to the target cluster.
    '--git-url'     set the URL for the git repository from which Kraan should pull AddonsLayer configs.
    '--git-user'    set (or override) the GIT_USER environment variables.
    '--git-token'   set (or override) the GIT_CREDENTIALS environment variables.
    '--no-git-auth' to indicate that no authorization is required to access git repository'
    '--toolkit'     to generate GitOps toolkit components.
    '--debug'       for verbose output.
    '--dry-run'     to generate yaml but not deploy to cluster. This option will retain temporary work directory.

To deploy to a cluster build the docker image and then deploy to the cluster. The following will build the docker image and push it to the Github packages then deploy to your Kubernetes cluster.
    
    export VERSION=v0.1.xx
    make clean-build
    make build
    docker login docker.pkg.github.com -u <github user> -p <github token>
    make docker-push
    scripts/setup.sh --kraan-version $VERSION --kraan-image-pull-secret auto

To deploy the image to your account in docker.io:

    make clean-build
    export REPO=<docker user>
    make build
    docker login -u <docker user> -p <docker password>
    make docker-push
    scripts/setup.sh --kraan-version $VERSION --no-gitops --kraan-image-repo $REPO

Set `REPO` to `kraan` to push to the `kraan` organisation in docker.io.

## Testing

To test the kraan-controller you can run it on your local machine against a kubernetes cluster:

    scripts/run-controller.sh --help

    USAGE: run-controller.sh [--debug]
    Run the Kraan Addon Manager on local machine
    options:
    '--debug' for verbose output
    This script will create a temporary directory and copy the addons.yaml and addons-source.yam files from testdata/addons to
    the temporary directory. It will then set the environmental variable DATA_PATH to the temporary directory. This will cause the
    kraan-controller to process the addons layers using the temporary directory as its root directory when storing files it retrieves
    from this git repository's testdata/addons directory using the source controller.

When you run the 'run-controller.sh' script it will copy some files to a temporary directory, emmit text as shown below and then prompt you as follow:

    Running kraan-controller with DATA_PATH set to /tmp/kraan-XJQUWB
    You may change files in /tmp/kraan-XJQUWB/testdata/addons to test kraan-controller
    Edit and then kubectl apply /tmp/kraan-XJQUWB/testdata/addons/addons.yaml to cause kraan-controller to reprocess layers.
    Edit and then kubectl apply /tmp/kraan-XJQUWB/testdata/addons/addons-source.yaml to cause kraan-controller to reprocess source controller data.
    In order to allow for this scenario the temporary directory will not be deleted so you are responsible for deleting this directory

    if you want change and rerun the kraan-controller you should type...
    export DATA_PATH=/tmp/kraan-XJQUWB
    kubectl -n gotk-system port-forward svc/source-controller 8090:80 &
    export SC_HOST=localhost:8090
    kraan-controller
    Pausing to allow user to make manual changes to testdata in /tmp/kraan-XJQUWB/testdata/addons, press enter to continue

The kraan-controller will reprocess all AddonsLayers perioidically. This period defaults to 30 seconds but can be set using a command line argument.

    kraan-controller -sync-period=1m

The reprocessing period can also be set to a period in seconds using the 's' suffix, i.e. 20s.

The `SC_TIMEOUT` environmental variable can be used to set the timeout period for retrieving data from the source controller, default is 15 seconds.

    export SC_TIMEOUT=30s

The `SC_HOST` environmental variable can be used to set the host component of the source controller's artifact url. This is useful when running the kraan-controller out of cluster to enable it to access the source controller via a local address using kubectl port-forward, i.e.

    kubectl -n gotk-system port-forward svc/source-controller 8090:80 &
	export SC_HOST=localhost:8090

If you elected to use the `--no-testdata` option when setting up the cluster then you will need to apply these files. You can do this by applying `.testdata/addons/addons-source.yaml` and `.testdata/addons/addons.yaml` to deploy the source controller custom resource and AddonsLayers custom resources respectively. This will cause the kraan-controller to operate on the testdata in the `./testdata` directory of this repository using the `master` branch. If you want to test against other branches use the copy of these files the `scripts/run-controller.sh` creates. Edit then apply those files.

If you want to use the kraan-controller to deploy items defined in your own repository edit the 'addons-source.yaml' file in the temporary directory to reference the repository and branch containing your addons definitions and apply it to the cluster. Then edit the `.testdata/addons/addons.yaml` file to define the addons layers and apply that.

### Integration Tests

To run integration tests:

    make integration

