# Changelog

## v0.3.55

### Dependency Bumps

**Go module updates:**
- `github.com/fluxcd/helm-controller/api` v1.1.0 → v1.5.4
- `github.com/fluxcd/source-controller/api` v1.4.1 → v1.8.3
- `github.com/fluxcd/pkg/apis/meta` v1.25.1 → v1.26.0
- `go.uber.org/zap` v1.27.1 → v1.28.0
- `golang.org/x/mod` v0.32.0 → v0.35.0
- `k8s.io/*` upgraded to v0.35.2
- `sigs.k8s.io/controller-runtime` v0.19.0 → v0.23.1

**Chart image tag updates:**
- `gotk.helmController` image tag: v1.5.3 → v1.5.4
- `gotk.sourceController` image tag: v1.8.1 → v1.8.3

### Issues Fixed

- **Kraan not recognizing `healthCheckExprs` on HelmRelease**: The helm-controller/api at v1.1.0
  did not include the `healthCheckExprs` field (CEL-based custom health checks). Bumping to v1.5.4
  brings full support for CEL health check expressions, enabling kraan to properly parse and
  validate HelmRelease objects that use this feature.

### Notable Behavior Changes

#### Server-Side Apply (SSA) is now the default

Helm v4 in helm-controller v1.5.x changes the default apply method from **client-side apply** to
**server-side apply (SSA)**. This affects all HelmReleases where `serverSideApply` is not explicitly set:

- The helm-controller becomes the **field owner** of resources it manages
- Conflicts are detected at apply time if another controller manages the same fields
- The `kubectl.kubernetes.io/last-applied-configuration` annotation is no longer used

**To opt out per-release:**
```yaml
spec:
  install:
    serverSideApply: false
  upgrade:
    serverSideApply: disabled
```

**To opt out globally**, add to `gotk.helmController.extraArgs`:
```
--feature-gates=UseHelm3Defaults=true
```

#### Drift Detection

Drift detection is **not** automatically enabled with Helm v4 defaults. It requires explicit
opt-in per HelmRelease:

| Mode | Behavior |
|------|----------|
| `enabled` | Detects drift and **corrects** it on reconcile |
| `warn` | Detects drift and emits **events/alerts** but does not correct |
| `disabled` (default) | No drift detection |

**To enable drift detection per-release:**
```yaml
spec:
  driftDetection:
    mode: enabled
```

**To ignore specific fields from drift detection:**
```yaml
spec:
  driftDetection:
    mode: enabled
    ignore:
      - paths: ["/spec/replicas"]
        target:
          kind: Deployment
```

> **Note:** Disabling drift detection does not disable SSA. You can use SSA (for conflict
> detection at apply time) without ongoing drift correction.

#### API Removals

- **`v2beta1` and `v2beta2` HelmRelease APIs are removed** — the CRD migration job (targeting
  `v2`) handles this automatically during helm upgrade.
- **`v1beta2` source APIs are removed** in source-controller v1.8.x — the CRD migration job
  (targeting `v1`) handles this automatically during helm upgrade.

#### New Feature Gates Available

The following feature gates can be enabled via `gotk.helmController.extraArgs`:

- `CancelHealthCheckOnNewRevision` — Cancel health checks when a new reconciliation starts
- `DefaultToRetryOnFailure` — Default retry strategy for failed releases
- `DirectSourceFetch` — Bypass cache for source objects
- `DisableConfigWatchers` — Disable ConfigMap/Secret watchers
- `ObjectLevelWorkloadIdentity` — Workload identity per HelmRelease object

## v0.3.54

- Upgraded source-controller from v1.7.4 to v1.8.1
- Upgraded helm-controller from v1.4.5 to v1.5.3
- Added CEL validation samples for health check expressions testing
