PROJECT:=kraan-controller
# Controller Integration test setup
export USE_EXISTING_CLUSTER?=true
export IMAGE_PULL_SECRET_SOURCE?=${HOME}/gotk-regcred.yaml
export IMAGE_PULL_SECRET_NAME?=gotk-regcred
export GITOPS_USE_PROXY?=auto
export KRAAN_NAMESPACE?=gotk-system
export KUBECONFIG?=${HOME}/.kube/config
export DATA_PATH?=$(shell mktemp -d -t kraan-XXXXXXXXXX)
export SC_HOST?=localhost:8090
export HTTPS_PROXY?=""
export NO_PROXY?=""
export no_proxy?=""