# Developers Guide

This readme contains guidance for developers. covering building the kraan-controller, setting up a development environment and testing the kraan-controller.

## Setup

clone [[Kraan repository]](https://github.com/fidelity/kraan) or fork it and clone your fork.

This project requires the following software:

    golangci-lint version = 1.38.0
    golang version >= 1.14.6
    mockgen = v1.4.4
    kubebuilder = v2.3.1
    flux = latest
    helm - v3.3.4

You can install [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder), [golangci-lint](https://github.com/golangci/golangci-lint), [mockgen](https://github.com/golang/mock), [helm](https://helm.sh/), [kind](https://kind.sigs.k8s.io/docs/) and [flux](https://toolkit.fluxcd.io/) using the 'setup.sh' script:

    bin/setup.sh

## Build

The Makefile in the project's top level directory will compile, build and test all components.

    make

Alternatively you can run make in a docker container.

    make check

If you change the golang source in the `api` directory such that the custom resource definition is changed you need to rin `make manifests` to regenerate the crd yaml in `config/crd/bases/kraan.io_addonslayers.yaml` and then cut an paste the contents of this file into `chart/templates/kraan/crd.yaml`, retaining the first and last line of this file.

A shell script is provided to perform `goimports`, `gofmt`, run linter and tests on a per package basis.

    bin/package.sh  <relative path to package>

For example to run this on the `kubectl` package:

    bin/package.sh pkg/internal/kubectl

To build docker image type:

    export REPO=<name of docker repository>
    export VERSION=<version> # use to generate the image tag
    make build

If you require a proxy to access the internet set the `DOCKER_BUILD_PROXYS` environmental variable before doing 'make build', e.g.

    export DOCKER_BUILD_PROXYS="--build-arg HTTP_PROXY=$HTTP_PROXY --build-arg HTTPS_PROXY=$HTTPS_PROXY"

For security reasons the image created uses `gcr.io/distroless/static:latest` with user `nobody`. When debugging issues it is sometimes useful to be able to access a shell in the pod container and install additional tools. To facilitate this a `dev-build` make target is provided which builds an image using `ubuntu` and allows root access. When deploying this development image use the `--kraan-dev` option when running the `scripts/deploy.sh` utility (see Deployment section below) to allow writes to the container filesystem.

Then to deploy the docker container:

    docker login <docker repository>
    make docker-push

If the `VERSION` is not set `master` is used. The REPO will default to `docker.pkg.github.com/fidelity/kraan` which only users with privileges on the `fidelity` organization will be able to push to. However if you fork the Kraan repository then the default will be `docker.pkg.github.com/<org/user you forked to>/kraan`.

## Releases

The offical releases use docker hub repository: kraan/kraan-controller. The 'master' tag will be contain a version of kraan built from the master branch. Release versions will be deployed to this repository for public use.

Helm Charts are deployed to github pages so can be accessed via repo: kraan <https://fidelity.github.io/kraan/>

    helm repo add kraan https://fidelity.github.io/kraan/
    helm search repo --regexp kraan --versions

## Creating a Release

To create a release set `VERSION` to the Kraan-Controller version, `REPO` to 'kraan' and `CHART_VERSION` to the chart version, then build and push the image:

    export REPO=kraan
    make clean-build
    make build
    make docker-push
    export CHART_VERSION=v0.1.xx
    make release

The 'release' target will update the tag in the values file, package the chart and deploy it to gh-pages.

Finally use github web interface to create a tag.

## Updating Gitops Toolkit Components

To update the gitops toolkit components (Helm Controller and Source Controller) run the deploy.sh script with the '--toolkit' option

    scripts/deploy.sh --toolkit

Then compare the file produced with the equivalent sections in the chart directory and updat the chart files accordingly.
Once the changes are tested and merged into master, create a new chart release as described above.

## Continuous Integration

- If you update the [`./VERSION`](./VERSION) a new release will be made according to the below scenarios
- If the docker release image version already exists, we DO NOT overwrite the image or chart. Prereleases get overwritten.
- Any PR against `master` automatically triggers a lint, test, build, and prerelease of the container image tag version `kraan/kraan-controller-prerelease:<VERSION>`.
- Any git push or merge into `master` triggers a lint, test, build, and release of the container image tag and helm chart version `kraan/kraan-controller:<VERSION>` if `VERSION` has changed.

## Deployment

A shell script is provided to deploy the artifacts necessary to test the kraan-controller to a kubernetes cluster. It does this using a helm client to install a helm chart containing the Kraan Controller and the GitOps Toolkit (flux) components it uses.

    https://github.com/fidelity/kraan.git
    USAGE: deploy.sh [--debug] [--dry-run] [--toolkit] [--deploy-kind] [--testdata] [--helm <upgrade| install>]
        [--kraan-image-reg <registry name>] [--kraan-image-repo <repo-name>] [--kraan-image-tag] [--kraan-dev]
        [--kraan-image-pull-secret auto | <filename>] [--gitops-image-pull-secret auto | <filename>]
        [--gitops-image-reg <repo-name>] [--kraan-loglevel N] [--prometheus <namespace>] [--values-files <file names>]
        [--gitops-proxy auto | <proxy-url>] [--git-url <git-repo-url>] [--no-git-auth]
        [--git-user <git_username>] [--git-token <git_token_or_password>]

    Install the Kraan Addon Manager and gitops source controller to a Kubernetes cluster

    Options:
    '--helm' perform helm upgrade or install, if you don't specify either script will not deploy helm chart.
    '--kraan-image-pull-secret' set to 'auto' to generate image pull secrets from ~/.docker/config.json
                                or supply name of file containing image pull secret defintion to apply.
                                The secret should be called 'kraan-regcred'.
    '--kraan-image-reg'   provide image registry to use for Kraan, defaults to empty for docker hub
    '--kraan-image-repo'  provide image repository prefix to use for Kraan, defaults to 'kraan' for docker hub org.
    '--kraan-image-name'  the name of the kraan image to use, defaults to kraan-controller.
    '--kraan-tag'         the tag of the kraan image to use.
    '--kraan-loglevel'    loglevel to use for kraan controller, 0 for info, 1 for debug, 2 for trace.
    '--kraan-dev'         select development mode, makes pod filesystem writable for debugging purposes.

    '--gitops-image-reg'  provide image registry to use for gitops toolkit components, defaults to 'ghcr.io'.
    '--gitops-image-pull-secret' set to 'auto' to generate image pull secrets from ~/.docker/config.json
                                or supply name of file containing image pull secret defintion to apply.
                                The secret should be called 'gotk-regcred'.
    '--gitops-proxy'    set to 'auto' to generate proxy setting for gotk components using value of HTTPS_PROXY
                        environment variable or supply the proxy url to use.
    '--values-files'    provide a comma separated list of yaml files containing values you want to set.
                        see samples directory for examples.

    '--deploy-kind' create a new kind cluster and deploy to it. Otherwise the script will deploy to an existing
                    cluster. The KUBECONFIG environmental variable or ~/.kube/config should be set to a cluster
                    admin user for the cluster you want to use. This cluster must be running API version 16 or
                    greater.
    '--prometheus'  install promethus stack in specified namespace
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
    scripts/deploy.sh --kraan-tag $VERSION --kraan-image-reg docker.pkg.github.com/fidelity --kraan-image-pull-secret auto

To deploy the image to your account in docker.io:

    make clean-build
    export REPO=<docker user>
    make build
    docker login -u <docker user> -p <docker password>
    make docker-push
    scripts/deploy.sh --kraan-tag $VERSION --kraan-image-repo $REPO

Set `REPO` to `kraan` to push to the `kraan` organisation in docker.io.

## Testing

To test the kraan-controller you can run it on your local machine against a kubernetes cluster:

    scripts/run-controller.sh --help

    USAGE: run-controller.sh [--log-level N] [--debug]
    Run the Kraan Addon Manager on local machine
    options:
    '--log-level' N, where N is 1 for debug message and 2 or higher for trace level debugging
    '--debug' for verbose output
    This script will create a temporary directory call /tmp/kraan-local-exec which the kraan-controller
    will use as its root directory when storing files it retrieves from this git repository

The kraan-controller will reprocess all AddonsLayers perioidically. This period defaults to 1 minute but can be set using a command line argument.

    kraan-controller -sync-period=2m

The reprocessing period can also be set to a period in seconds using the 's' suffix, i.e. 20s.

The `SC_TIMEOUT` environmental variable can be used to set the timeout period for retrieving data from the source controller, default is 15 seconds.

    export SC_TIMEOUT=30s

The `SC_HOST` environmental variable can be used to set the host component of the source controller's artifact url. This is useful when running the kraan-controller out of cluster to enable it to access the source controller via a local address using kubectl port-forward, i.e.

    kubectl -n gotk-system port-forward svc/source-controller 8090:80 &
    export SC_HOST=localhost:8090

If you elected to use the `--testdata` option when setting up the cluster test data wil be added. Alternatively, you can do this by applying `.testdata/addons/addons-source.yaml` and `.testdata/addons/addons.yaml` to deploy the source controller custom resource and AddonsLayers custom resources respectively. This will cause the kraan-controller to operate on the testdata in the `./testdata` directory of this repository using the `master` branch.

### Integration Tests

To run integration tests:

    make integration

If running with proxy access to github

    export GITOPS_USE_PROXY?=auto

If image pull secret is required to access public repositories, provide a image pull secret file

    export IMAGE_PULL_SECRET_SOURCE?=${HOME}/gotk-regcred.yaml
    export IMAGE_PULL_SECRET_NAME?=gotk-regcred

## Debugging

The Kraan Controller emits json format log records. Most log record contains a number of common fields that can be used to select log messages to display when examining log data.

| field name | Description                                                                                                                                                                                          |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| function   | The function name, in format `package.interface/object.method/function`                                                                                                                              |
| source     | The source file, just file name, not full path name                                                                                                                                                  |
| line       | The line number                                                                                                                                                                                      |
| kind       | The kind for log messages relating to owned or watched objects                                                                                                                                       |
| layer      | The layer name for log messages relating to addons layers                                                                                                                                            |
| msg        | The message text                                                                                                                                                                                     |
| logger     | name of the logger, one of `controller-runtime.manager`, `controller-runtime`, `metrics`, `initialization`, `kraan.controller.applier`, `kraan.controller.reconciler` or `kraan.manager.controller`  |
| level      | Level of message. This will be `info`, `debug` or `Level(-N)`, where 'N' is the trace level. Levels 2 to 4 are currently used.<br>Level 4 is exclusively used by tracing of function entry and exit. |
| ts         | timestamp of log message                                                                                                                                                                             |
