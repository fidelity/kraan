#!/bin/sh
export VERSION=test
export REPO=kraan
make clean-build
make build
kind create cluster
kind load docker-image kraan/kraan-controller:test
helm upgrade -i kraan chart --set kraan.kraanController.image.tag=test

kubectl apply -f ../gitops-examples/helm-repos
kubectl apply -f ../gitops-examples/git-repos
kubectl apply -f ../gitops-examples/kraan
