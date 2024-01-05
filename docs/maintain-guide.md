# Maintenance Guide

Guidance for kraan maintenance.

### 1. FluxCD dependencies upgrade

Check latest release: 
* __CRD__ update, copy latest CRD for [source-controller](https://github.com/fluxcd/source-controller/tree/main/config/crd/bases) and [helm-controller](https://github.com/fluxcd/helm-controller/blob/main/config/crd/bases/helm.toolkit.fluxcd.io_helmreleases.yaml)
* __Deployment__ update for [source-controller](https://github.com/fluxcd/source-controller/tree/main/config/manager) and [helm-controller](https://github.com/fluxcd/helm-controller/tree/main/config/manager)
* Update __Image version__ in [values.yaml](https://github.com/fidelity/kraan/blob/master/chart/values.yaml)
* Update __flux dependencies__ in [go.mod](https://github.com/fidelity/kraan/blob/master/go.mod), fix compile errors.

### 2. Kubernetes version upgrade

Check target kubernetes version: 
* upgrade __kubernetes dependencies__ version in [go.mod](https://github.com/fidelity/kraan/blob/master/go.mod)
* upgrade __kubectl__ version in [Dockerfile](https://github.com/fidelity/kraan/blob/master/Dockerfile)
* upgrade __integration testing cluster__ version in [workflow.yaml](https://github.com/fidelity/kraan/blob/master/.github/workflows/main.yaml)

### 3. Dependencies upgrade

Check __dependabot PRs__, upgrade dependencies version in [go.mod](https://github.com/fidelity/kraan/blob/master/go.mod) file. Run `go mod tidy`, fix compile errors.

### 4. Golang version upgrade

Check Golang version, upgrade to latest Golang version, in [go.mod](https://github.com/fidelity/kraan/blob/master/go.mod), [Dockerfile](https://github.com/fidelity/kraan/blob/master/Dockerfile), [workflow.yaml](https://github.com/fidelity/kraan/blob/master/.github/workflows/main.yaml)

### 5. Local test

* `go test ./...`
* `golangci-lint run ./...`
* Fix failed tests and lint errors

### 5. Local integration test

* Connect to a cluster, `kubectl scale --replicas=0 deployment/kraan-controller`.
* `export SC_HOST=localhost:8090`
* Start local kraan-controller.
* Delete and re-apply __AL__, check if can be reconciled successfully.

### 6. Github pipeline

* __Actions__ executed successfully
* __Checks__ passed
* __Image__ and __Chart__ published successfully

### 7. Testing in cluster

* Connect to a cluster
* Install latest chart.
* Delete and re-apply __Gitrepo__, __helmrepo__, __AL__, __HR__, check if can be reconciled successfully.