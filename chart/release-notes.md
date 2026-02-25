# Release Notes

## v0.3.53 (2026-02-23)

### Summary
- Added a pre-upgrade migration job for HelmRelease CRD stored versions.


### Notes
- The script annotates HelmReleases, checks CRD stored versions, and patches status to v2 when migration is complete.

