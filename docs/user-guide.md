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
`global.prometheusAnnotations` | Annotations to be applied to controllers to allow prometheus server scrapping. | `prometheus.io/scrape: "true"`<br> `prometheus.io/port: "8080"`
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
`kraan.kraanController.args.syncPeriod` | The period between reprocessing of all AddonsLayers | `1m`
`kraan.kraanController.devmode` | set to true when running a development image to allow writes to container filesystem | `false`
`kraan.kraanController.resources` | resource settings for `kraan-controller` | `limits:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`cpu: 1000m`<br>&nbsp;&nbsp;&nbsp;&nbsp;`memory: 1Gi`<br>`requests:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`cpu: 500m`<br>&nbsp;&nbsp;&nbsp;&nbsp;`memory: 128Mi`
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
`gotk.sourceController.resources` | resource settings for `source-controller` | `limits:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`cpu: 1000m`<br>&nbsp;&nbsp;&nbsp;&nbsp;`memory: 1Gi`<br>`requests:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`cpu: 500m`<br>&nbsp;&nbsp;&nbsp;&nbsp;`memory: 128Mi`
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
`gotk.helmController.resources` | resource settings for `helm-controller` | `limits:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`cpu: 1000m`<br>&nbsp;&nbsp;&nbsp;&nbsp;`memory: 1Gi`<br>`requests:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`cpu: 500m`<br>&nbsp;&nbsp;&nbsp;&nbsp;`memory: 128Mi`
`gotk.helmController.tolerations` | tolerations for `helm-controller` | `{}`
`gotk.helmController.nodeSelector` | nodeSelector settings for `helm-controller` | `{}`
`gotk.helmController.affinity` | affinity settings for `helm-controller` | `{}`

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install` or use the `--values` to specify a comma seperated list of values files. The `samples` directory contains some example values files.
# Usage

The Kraan-Controller is designed to deploy [HelmReleases](https://toolkit.fluxcd.io/guides/helmreleases/) to a Kubernetes cluster. Using `addonslayers.kraan.io` custom resources it retrieves the definitions of one or more HelmReleases from a git repository and applies them to the cluster then waits until they are all successfully deployed before marking the AddonsLayer as `Deployed`. My creating multiple AddonsLayers custom resources and setting the dependencies between them the user can sequence the deployment of multiple layers of addons.

## Git Repository Source

An AddonsLayer references a `gitrepository.source.toolkit.fluxcd.io` custom resource which it uses to retrieve data from a git repository using the Source-Controller. The `source` element of the AddonsLayer custom resource references a GitRepository custom resource the. The `path` element under `source` is the path relative to the top directory of the git repository referenced by the GitRepository custom resource where the HelmReleases comprising this AddonsLayer are defined.
## Kubernetes Version Prerequite

An AddonsLayer can also optionally include a `prereqs` element containing the minimum version of the Kubernetes API required by the AddonsLayer. If specified, the AddonsLayer will not be applied until the cluster API version is greater than or equal to the specified version. The Kraan-Controller will regularly check the Cluster API version.

## Processing Controls

The `interval` field is used to specify the period to wait before reprocessing an AddonsLayer. Note that all AddonsLayers are reprocessed periodically. The period between reprocessing of all AddonsLayers defaults to one minute but can set using the `syncPeriod` value, see Configuration section above.

The `hold` setting can be used to prevent processing of the AddonsLayer. Set to `true` to enable this feature.
## Versions

The `version` field defines the version of the AddonsLayer. This can be used to define a new version of the AddonsLayer. Changing the version affects other AddonsLayers that are dependent on this layer. If you change the version of an AddonsLayer you need to update the version in `dependsOn` field in the dependent layer to make that layer dependent on the new version of this layer.

Modifying the version field can be used to force redeployment of HelmReleases that have not changed. This feature can be activated by adding an annotation to the HelmRelease definition. Setting `kraan.updateVersion: "true"` will cause the Kraan-Controller to add a value to that HelmRelease with key `kraanVersion` and value of the AddonsLayer's version. This means that if the version has changed since the last time the AddonsLayer was processed the HelmRelease will be redeployed. This feature enables the user configure an integration test HelmRelease for an AddonsLayer which will be run on AddonsLayer version change even if it has not changed. By using the HelmRelease `dependsOn` feature you can ensure this HelmRelease is not deployed until all other HelmReleases in the layer are deployed, see [testdata/addons/bootstrap](https://github.com/fidelity/kraan/tree/master/testdata/addons/bootstrap) for an example.

The version field does not need to be updated, simply commiting a change to the git repository branch referenced by the GitRepository custom resource that is referenced in the AddonsLayer's source element or editing that GitRepository to reference a different tag, commit or even a different git repository will cause Kraan to reprocess the AddonsLayer.

## Example AddonsLayers

```yaml
apiVersion: kraan.io/v1alpha1
kind: AddonsLayer
metadata:
  name: bootstrap
spec:
  version: 0.1.01
  hold: false
  interval: 1m
  source:
    name: addons-config
    namespace: gotk-system
    path: ./testdata/addons/bootstrap
  prereqs:
      k8sVersion: "v1.16"
---
apiVersion: kraan.io/v1alpha1
kind: AddonsLayer
metadata:
  name: base
spec:
  version: 0.1.01
  interval: 1m
  source: 
    name: addons-config
    namespace: gotk-system
    path: ./testdata/addons/base
  prereqs:
      k8sVersion: "v1.16"
      dependsOn:
        - bootstrap@0.1.01
```

## Pruning

The Kraan-Controller monitors any `hemrelease.helm.toolkit.fluxcd.io` resources owned by a AddonsLayer and reprocesses the AddonsLayer when it detects changes to the HelmReleases it owns. This means that if a HelmRelease is deleted or amended using kubectl or other cluster management tools, Kraan-Controller will redeploy it using the definition in the git repository references by the AddonsLayer source field. To prevent this, set the hold field in the AddonsLayer to `true`.

When processing an AddonsLayer the Kraan-Controller first `prunes` any HelmReleases owned by the AddonsLayer that are not defined in the git repository. This means that removing a HelmRelease definition from a git repository will cause it to be uninstalled from the cluster.

Kraan-Controller performs the prune processing on all AddonLayers without reference to layer dependencies. It then proceeds with deploying addons, waiting for any layers it depends on to be deployed first. This enables a user to move a HelmRelease from one layer to another without risk of deployment failing because the previous layer's deployment is still present. This would potentially occur if a HelmRelease is moved to a layer that its current layer depends on. By performing the pruning first this potential issue is avoided. However, the Kraan-Controller does not wait for pruning of all layers to be completed before proceeding with layer deployment, although it does wait for each layer to be pruned before deploying that layer. This means a deployment might fail because a previous version (defined in another layer) of the HelmRelease has not completed pruning yet. Hoever this issue will be resolved when the pruning has completed.
