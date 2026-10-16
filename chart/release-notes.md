# Release Notes
## v0.3.57
- update kraan chart version and appVersion to v0.3.57
- update go packages
- update source-controller to v1.9.6 and helm-controller to v1.6.5 with accompanying CRD updates

### ⚠️ Breaking changes
- helm-controller v1.6.0: the default `HelmRelease` `.spec.postRenderStrategy` changed from
  `nohooks` (Helm v3 behavior) to `combined` (Helm v4.2 default), changing how hooks are sent to
  post-renderers. Set `.spec.postRenderStrategy: nohooks` on affected HelmReleases, or enable the
  `UseHelm3Defaults` feature gate, to retain the previous behavior.
- helm-controller v1.6.4: `HelmRelease` `kubeConfig` Secrets that reference local files are no
  longer supported; credentials and certificates must be embedded inline in the Secret.

## v0.3.56
- upgrade source-controller to v1.8.5 and helm-controller to v1.5.5 with accompanying CRD updates
- update kraan chart version and appVersion to v0.3.56

## v0.3.54
- upgrade source-controller to v1.8.1 and helm-controller to v1.5.3 with accompanying CRD updates
- add integration test cases for CEL health check expressions on HelmReleases

## v0.3.53 (2026-02-23)

### Summary
- Added a pre-upgrade migration job for HelmRelease CRD stored versions.


### Notes
- The script annotates HelmReleases, checks CRD stored versions, and patches status to v2 when migration is complete.
