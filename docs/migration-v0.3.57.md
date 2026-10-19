# Migration Guide: v0.3.57 (helm-controller v1.6.5, source-controller v1.9.6)

This release bumps `helm-controller` from v1.5.5 to v1.6.5 and `source-controller` from
v1.8.5 to v1.9.6. Two behavioral changes introduced in helm-controller require action if
your `HelmRelease` resources rely on the old behavior.

## 1. `postRenderStrategy` default change (helm-controller v1.6.0)

The default `HelmRelease` `.spec.postRenderStrategy` changed from `nohooks` (Helm v3
behavior) to `combined` (Helm v4.2 default). This changes how hook manifests are sent to
post-renderers.

**Before (implicit old behavior)** — nothing needed to be set, `nohooks` was the default.

**After upgrade**, if you rely on the old behavior, pin it explicitly on the `HelmRelease`:

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: my-app
spec:
  chart:
    spec:
      chart: my-chart
  postRenderStrategy: nohooks   # restores pre-v1.6.0 behavior
```

**Or**, restore the old default for *all* `HelmRelease` resources cluster-wide by enabling
the feature gate on the helm-controller deployment (via `values.yaml` extraArgs):

```yaml
args:
- --feature-gates=UseHelm3Defaults=true
```

If you don't use post-render hooks at all, no action is needed — this only matters when
`.spec.postRenderers` with hooks are in play.

## 2. Local-file `kubeConfig` Secrets rejected (helm-controller v1.6.4)

`HelmRelease.spec.kubeConfig` Secrets that reference on-disk file paths for
certificates/keys are no longer accepted. Credentials and certificates must be embedded
inline in the Secret.

**Before (now rejected)**:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: remote-cluster-kubeconfig
stringData:
  value: |
    apiVersion: v1
    kind: Config
    clusters:
    - cluster:
        server: https://remote-api:6443
        certificate-authority: /etc/certs/ca.pem        # ❌ local file path
      name: remote
    users:
    - name: remote-user
      user:
        client-certificate: /etc/certs/client.pem       # ❌ local file path
        client-key: /etc/certs/client-key.pem           # ❌ local file path
```

**After** — embed the cert/key/CA content inline as base64 (`-data` fields):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: remote-cluster-kubeconfig
stringData:
  value: |
    apiVersion: v1
    kind: Config
    clusters:
    - cluster:
        server: https://remote-api:6443
        certificate-authority-data: <base64 CA cert>      # ✅ inline
      name: remote
    users:
    - name: remote-user
      user:
        client-certificate-data: <base64 client cert>     # ✅ inline
        client-key-data: <base64 client key>              # ✅ inline
```

Generate the base64 values with `base64 -i ca.pem | tr -d '\n'`, or if you're exporting an
existing kubeconfig, `kubectl config view --raw --flatten` already embeds the data inline.

This only affects `HelmRelease.spec.kubeConfig` Secrets used to target a remote cluster —
not the controller's own in-cluster kubeconfig.

## References

- [helm-controller CHANGELOG](https://github.com/fluxcd/helm-controller/blob/main/CHANGELOG.md)
- [source-controller CHANGELOG](https://github.com/fluxcd/source-controller/blob/main/CHANGELOG.md)
- [CHANGELOG.md](../CHANGELOG.md)
- [chart/release-notes.md](../chart/release-notes.md)
