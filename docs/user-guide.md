# User Guide

This readme contains guidance for users relating to the deployment and use of Kraan.

# Deployment

```console
helm install kraan kraan/kraan-controller --values samples/datadog.yaml,samples/local.yaml --namespace gotk-system
```
