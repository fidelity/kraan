# User Guide

This readme contains guidance for users relating to the deployment and use of Kraan.

# Deployment

```console
helm repo add kraan https://fidelity.github.io/kraan
helm install kraan kraan/kraan-controller --namespace gotk-system
```

## Introduction

This chart deploys a [Kraan](https://fidelity.github.io/kraan/) and the [Gitops Toolkit](https://toolkit.fluxcd.io/) components it uses on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.16+

## Installing the Chart

To install the chart with the release name `kraan` in namespace `gotk-system`:

```console
kubectl create namespace gotk-system
helm install kraan kraan/kraan-controller --namespace gotk-system
```

## Uninstalling the Chart

To uninstall the `kraan` deployment:

```console
helm uninstall kraan
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following table lists the configurable parameters of the Prometheus chart and their default values.

Parameter | Description | Default
--------- | ----------- | -------
`global.extraPodAnnotations` | list of annotations to add to `kraan-controller`, `source-controller` and `helm-controller`, see [samples/local.yaml](https://github.com/fidelity/kraan/blob/master/samples/local.yaml) for example. | `{}`
`global.extraLabels` | list of labels to add to `kraan-controller`, `source-controller` and `helm-controller` | `{}`
`global.prometheusAnnotations` | Annotations to be applied to controllers to allow prometheus server scrapping. | `prometheus.io/scrape: "true"` `prometheus.io/port: "8080"`
`global.env.httpsProxy` | HTTPS proxy url to use. |
`global.env.noProxy` | no proxy CIDRs. | `10.0.0.0/8,172.0.0.0/8`
`kraan.crd.enabled` | install/update Kraan CRDs | `true`
`kraan.rbac.enabled` | install/update Kraan RBAC rules | `true`
`kraan.netpolicy.enabled` | install/update Kraan network policies | `true`
`kraan.kraanController.enabled` | install/update Kraan Controller | `true`
`kraan.kraanController.name` | name of Kraan Controller | `kraan-controller`
`kraan.kraanController.extraPodAnnotations` | list of annotations to add to `kraan-controller` | `{}`
`kraan.kraanController.extraLabels` | list of labels to add to `kraan-controller` | `{}`
`kraan.kraanController.prometheus.enabled` | apply prometheus annotations to  `kraan-controller` | `true`
`kraan.kraanController.imagePullSecrets.name` | name of  Kraan Controller's `imagePullSecrets` |
`kraan.kraanController.image.repository` | Kraan Controller'srepository | `kraan`
`kraan.kraanController.image.tag` | Kraan Controller's image tag | `.chart.AppVersion`
`kraan.kraanController.args.logLevel` | Kraan Controller's log level, 0 for info, 1 for debug, 2 or greater for trace levels | `0`
`kraan.kraanController.devmode` | set to true when running a development image to allow writes to container filesystem | `false`
`kraan.kraanController.resources` | resource settings for `kraan-controller` | `limits:` `  cpu: 1000m` `  memory: 1Gi` `requests:` `  cpu: 500m` `   memory: 128Mi`
`kraan.kraanController.tolerations` | tolerations for `kraan-controller` | `{}`
`kraan.kraanController.nodeSelector` | nodeSelector settings for `kraan-controller` | `{}`
`kraan.kraanController.affinity` | affinity settings for `kraan-controller` | `{}`
`gotk.rbac.enabled` | install/update Kraan RBAC rules | `true`
`gotk.netpolicy.enabled` | install/update Kraan network policies | `true`
`gotk.sourceController.crd.enabled` | install/update Source Controller CRDs | `true`
`gotk.sourceController.enabled` | install/update Source Controller | `true`
`gotk.sourceController.name` | name of Source Controller | `source-controller`
`gotk.sourceController.extraPodAnnotations` | list of annotations to add to `source-controller` | `{}`
`gotk.sourceController.extraLabels` | list of labels to add to `source-controller` | `{}`
`gotk.sourceController.prometheus.enabled` | apply prometheus annotations to  `source-controller` | `true`
`gotk.sourceController.imagePullSecrets.name` | name of  Source Controller's `imagePullSecrets` |
`gotk.sourceController.image.repository` | Source Controller'srepository | `ghcr.io/fluxcd`
`gotk.sourceController.image.tag` | Source Controller's image tag | `v0.2.1`
`gotk.sourceController.resources` | resource settings for `source-controller` | `limits:` `  cpu: 1000m` `  memory: 1Gi` `requests:` `  cpu: 500m` `   memory: 128Mi`
`gotk.sourceController.tolerations` | tolerations for `source-controller` | `{}`
`gotk.sourceController.nodeSelector` | nodeSelector settings for `source-controller` | `{}`
`gotk.sourceController.affinity` | affinity settings for `source-controller` | `{}`
`gotk.helmController.crd.enabled` | install/update Helm Controller CRDs | `true`
`gotk.helmController.enabled` | install/update Helm Controller | `true`
`gotk.helmController.name` | name of Helm Controller | `helm-controller`
`gotk.helmController.extraPodAnnotations` | list of annotations to add to `helm-controller` | `{}`
`gotk.helmController.extraLabels` | list of labels to add to `helm-controller` | `{}`
`gotk.helmController.prometheus.enabled` | apply prometheus annotations to  `helm-controller` | `true`
`gotk.helmController.imagePullSecrets.name` | name of  Helm Controller's `imagePullSecrets` |
`gotk.helmController.image.repository` | Helm Controller'srepository | `ghcr.io/fluxcd`
`gotk.helmController.image.tag` | Helm Controller's image tag | `v0.2.0`
`gotk.helmController.resources` | resource settings for `helm-controller` | `limits:` `  cpu: 1000m` `  memory: 1Gi` `requests:` `  cpu: 500m` `   memory: 128Mi`
`gotk.helmController.tolerations` | tolerations for `helm-controller` | `{}`
`gotk.helmController.nodeSelector` | nodeSelector settings for `helm-controller` | `{}`
`gotk.helmController.affinity` | affinity settings for `helm-controller` | `{}`
Specify each parameter using the `--set key=value[,key=value]` argument to `helm install` or use the `--values` to specify a comma seperated list of values files. The `samples` directory contains some example values files.
