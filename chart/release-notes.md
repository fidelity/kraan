# Release Notes

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
