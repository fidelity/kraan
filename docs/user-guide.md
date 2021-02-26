# User Guide

This readme contains guidance for users relating to the deployment and use of Kraan.

## Deployment

```console
helm repo add kraan https://fidelity.github.io/kraan
helm install kraan kraan/kraan-controller --namespace gotk-system
```

### Introduction

This chart deploys a [Kraan](https://fidelity.github.io/kraan/) and the [Gitops Toolkit](https://toolkit.fluxcd.io/) components it uses on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

### Prerequisites

- Kubernetes 1.16+

### Installing the Chart

To install the chart with the release name `kraan` in namespace `gotk-system`:

```console
kubectl create namespace gotk-system
helm install kraan kraan/kraan-controller --namespace gotk-system
```

### Uninstalling the Chart

To uninstall the `kraan` deployment:

```console
helm uninstall kraan
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

### Configuration

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
`kraan.kraanController.image.repository` | Kraan Controller's repository | `kraan`
`kraan.kraanController.image.name` | Kraan Controller's image name | `kraan-controller`
`kraan.kraanController.image.tag` | Kraan Controller's image tag | `.chart.AppVersion`
`kraan.kraanController.image.imagePullPolicy` | Kraan Controller's image pull policy | InNotPresent
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
`gotk.sourceController.image.repository` | Source Controller's repository | `ghcr.io/fluxcd`
`gotk.sourceController.image.name` | Source Controller's image name | `source-controller`
`gotk.sourceController.image.tag` | Source Controller's image tag | `v0.7.1`
`gotk.sourceController.image.imagePullPolicy` | image pull policy | InNotPresent
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
`gotk.helmController.image.repository` | Helm Controller's repository | `ghcr.io/fluxcd`
`gotk.helmController.image.name` | Source Controller's image name | `helm-controller`
`gotk.helmController.image.tag` | Helm Controller's image tag | `v0.6.1`
`gotk.helmController.image.imagePullPolicy` | image pull policy | InNotPresent
`gotk.helmController.resources` | resource settings for `helm-controller` | `limits:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`cpu: 1000m`<br>&nbsp;&nbsp;&nbsp;&nbsp;`memory: 1Gi`<br>`requests:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`cpu: 500m`<br>&nbsp;&nbsp;&nbsp;&nbsp;`memory: 128Mi`
`gotk.helmController.tolerations` | tolerations for `helm-controller` | `{}`
`gotk.helmController.nodeSelector` | nodeSelector settings for `helm-controller` | `{}`
`gotk.helmController.affinity` | affinity settings for `helm-controller` | `{}`

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install` or use the `--values` to specify a comma seperated list of values files. The `samples` directory contains some example values files.

## Usage

The Kraan-Controller is designed to deploy [HelmReleases](https://toolkit.fluxcd.io/guides/helmreleases/) to a Kubernetes cluster. Using `addonslayers.kraan.io` custom resources it retrieves the definitions of one or more HelmReleases from a git repository and applies them to the cluster then waits until they are all successfully deployed before marking the AddonsLayer as `Deployed`. By creating multiple AddonsLayers custom resources and setting the dependencies between them the user can sequence the deployment of multiple layers of addons.

### Moving HelmReleases between AddonLayers

If a HelmRelease is removed from one AddonLayer's git repository source path and added to another layer's git repositiory source path then Kraan will detect this and not allow the existing HelmRelease on the cluster to be 'adpoted' by the layer it is now defined in rather than being deleted and readded. However to achieve this behaviour both AddonLayers need to be updated at the same time.

The 'prune' processing for the layer it has been removed from will mark the HelmReelease as 'orphaned' and wait for a period set by that layer's `interval` setting, which defauts to one minute, before uninstalling it. This allows an opportunity for another layer to identify that the HelmRelease in its git repository source path is present on the cluster but currently owned by another layer. In this case if the HelmRelease has been updated by the other layer with an 'orphaned' label this layer will change the owner and apply it to the cluster, which will cause the Helm Controller to upgrade it if any changes have been made or do nothing if the HelmRelease definition matches the current state on the cluster.

If a HelmRelease is defined in multiple layers at the same time the first layer to process it will acquire ownership and the other layer(s) will fail to apply. This will be clearly reported in the AddonLayer status. It is possible that this error could occur briefly in the sceanrio where a HelmRelease is moved from one layer to another if the layer it is now defined in processes before the layer that it has been removed from, but this situation will be resolved when the layer it has been removed from processes and adds an 'orphaned' label.

### Git Repository Source

An AddonsLayer references a `gitrepository.source.toolkit.fluxcd.io` custom resource which it uses to retrieve data from a git repository using the Source-Controller. The `source` element of the AddonsLayer custom resource references a GitRepository custom resource the. The `path` element under `source` is the path relative to the top directory of the git repository referenced by the GitRepository custom resource where the HelmReleases comprising this AddonsLayer are defined.

### Kubernetes Version Prerequite

An AddonsLayer can also optionally include a `prereqs` element containing the minimum version of the Kubernetes API required by the AddonsLayer. If specified, the AddonsLayer will not be applied until the cluster API version is greater than or equal to the specified version. The Kraan-Controller will regularly check the Cluster API version.

### Processing Controls

The `interval` field is used to specify the period to wait before reprocessing an AddonsLayer. Note that all AddonsLayers are reprocessed periodically. The period between reprocessing of all AddonsLayers defaults to one minute but can set using the `syncPeriod` value, see Configuration section above.

The `interval` field is also used to define the period to wait for another layer to adopt a HelmRelease that has been removed from a layer. See Pruning section below for more details.

The `timeout` field is used to set the period to wait for HelmReleases to be deployed before setting the AddonsLayer's status to failed.

The `hold` setting can be used to prevent processing of the AddonsLayer. Set to `true` to enable this feature.

### Versions

The `version` field defines the version of the AddonsLayer. This can be used to define a new version of the AddonsLayer. Changing the version affects other AddonsLayers that are dependent on this layer. If you change the version of an AddonsLayer you need to update the version in `dependsOn` field in the dependent layer to make that layer dependent on the new version of this layer.

Modifying the version field can be used to force redeployment of HelmReleases that have not changed. This feature can be activated by adding an annotation to the HelmRelease definition. Setting `kraan.updateVersion: "true"` will cause the Kraan-Controller to add a value to that HelmRelease with key `kraanVersion` and value of the AddonsLayer's version. This means that if the version has changed since the last time the AddonsLayer was processed the HelmRelease will be redeployed. This feature enables the user configure an integration test HelmRelease for an AddonsLayer which will be run on AddonsLayer version change even if it has not changed. By using the HelmRelease `dependsOn` feature you can ensure this HelmRelease is not deployed until all other HelmReleases in the layer are deployed, see [testdata/addons/bootstrap](https://github.com/fidelity/kraan/tree/master/testdata/addons/bootstrap) for an example.

The version field does not need to be updated, simply commiting a change to the git repository branch referenced by the GitRepository custom resource that is referenced in the AddonsLayer's source element or editing that GitRepository to reference a different tag, commit or even a different git repository will cause Kraan to reprocess the AddonsLayer.

### Example AddonsLayers

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

### Pruning

The Kraan-Controller monitors any `helmrelease.helm.toolkit.fluxcd.io` resources owned by a AddonsLayer and reprocesses the AddonsLayer when it detects changes to the HelmReleases it owns. This means that if a HelmRelease is deleted or amended using kubectl or other cluster management tools, Kraan-Controller will redeploy it using the definition in the git repository references by the AddonsLayer source field. To prevent this, set the hold field in the AddonsLayer to `true`.

When processing an AddonsLayer the Kraan-Controller first `prunes` any HelmReleases owned by the AddonsLayer that are not defined in the git repository. This means that removing a HelmRelease definition from a git repository will cause it to be uninstalled from the cluster.

Kraan-Controller performs the prune processing on all AddonLayers without reference to layer dependencies. It then proceeds with deploying addons, waiting for any layers it depends on to be deployed first. Before pruning HelmReleases the Kraan-Controller waits for a configurable period of time to see if the HelmRelease has been moved to another layer.
The `interval` field of the layer a HelmRelease has been removed from is used to define the period to wait for another layer to adopt a HelmRelease that has been removed from that layer. When a HelmRelease is removed from a layer an `orphaned` label containing the current date and time is added to the HelmRelease. The pruning process will be deferred for the `interval` period before proceeding with the uninstallation of the HelmRelease. This is designed to allow time for another layer to adopt the HelmRelease if it is now defined in its repository source. The HelmRelease is adopted by removing the orphaned label and changing the owner.

## Observability

The Kraan Controller generates Kubernetes Events for AddonsLayer custom resources. It also includes the status of the HelmReleases managed by an AddonsLayer in the custom resource status.

```console
kubectl describe al base
Name:         base
Namespace:
Labels:       <none>
Annotations:  ...
API Version:  kraan.io/v1alpha1
Kind:         AddonsLayer
Metadata:
  Creation Timestamp:  2020-11-30T10:31:11Z
  Finalizers:
    finalizers.kraan.io
  Generation:  4
  Managed Fields: ...
Spec:
  Interval:  1m
  Prereqs:
    Depends On:
      bootstrap@0.1.03
    k8sVersion:  v1.16
  Source:
    Kind:   gitrepositories.source.toolkit.fluxcd.io
    Name:   addons-config
    Path:   ./testdata/addons/base
  Timeout:  30s
  Version:  0.1.03
Status:
  Conditions:
    Last Transition Time:  2020-12-01T10:28:16Z
    Message:               AddonsLayer failed, HelmRelease: base/microservice-1, not ready
    Reason:                Failed
    Status:                True
    Type:                  Failed
  Observed Generation:     4
  Resources:
    Kind:                  helmreleases.helm.toolkit.fluxcd.io
    Last Transition Time:  2020-12-01T10:27:57Z
    Name:                  microservice-1
    Namespace:             base
    Status:                TestFailed
  Revision:                master/6a226b05a5aa0a775c2147d5b8b3b14d1adfa094
  State:                   Failed
  Version:                 0.1.03
Events:
  Type    Reason                              Age                 From              Message
  ----    ------                              ----                ----              -------
  ....
  Normal  Deployed                            47m                 kraan-controller  AddonsLayer version 0.1.03 is Deployed, All HelmReleases deployed
  Normal  ApplyPending                        37s                 kraan-controller  Waiting for layer: bootstrap, to apply source revision: main/5bfb0..... Layer: bootstrap, current state: Applying, deployed revision: master/6a226...
  Normal  Applying                            31s                 kraan-controller  AddonsLayer is being applied
  Normal  Failed                              1s (x2 over 23s)    kraan-controller  AddonsLayer failed, HelmRelease: base/microservice-1, not ready
```

A 'HelmRelease not deployed' log message will be emmitted if a HelmRelease fails to deploy. This message includes the layer name as well as HelmRelease namespace and name, e.g.

```json
{
  "level": "info",
  "ts": "2020-12-01T10:28:22.700Z",
  "logger": "kraan.controller.reconciler",
  "msg": "HelmRelease not deployed",
  "function": "controllers.(*AddonsLayerReconciler).setHelmReleaseFailed",
  "source": "addons_controller.go",
  "line": 417,
  "layer": "base",
  "name": "base/microservice-1"
}
```

The Kraan-Controller also generates metrics for `Deployed` and `Failed` conditions.
