# Helm addons

This directory contains cluter addons deployed from Helm charts.
The ArgoCD `helm-addons` ApplicationSet will generate an Application for each directory contained here with name equal to the directory name.

Each directory must contain a `config.yaml` and optionally a single `values.yaml` files for customizing the release.

The structure for the `config.yaml` file is as follows:

```yaml
repo: https://strimzi.io/charts
name: strimzi-kafka-operator
version: 0.49.1
namespace: strimzi-kafka-operator-system
```

The below table contains description of each key and an example value.

| Key         | Description                                                                  | Example                                                 |
| ----------- | ---------------------------------------------------------------------------- | ------------------------------------------------------- |
| repo        | URL of the Helm chart repo. OCI repos should not contain "oci://"            | https://strimzi.io/charts or quay.io/strimzi-helm (OCI) |
| name        | Name of the Helm chart in the repo                                           | strimzi-kafka-operator                                  |
| version     | Version of the Helm chart. SemVer constraints supported                      | 0.49.\*                                                 |
| namespace   | Namespace to deploy to                                                       | strimzi-kafka-operator-system                           |
| releaseName | (optional) Release name to pass to helm template. If unset, name key is used | strimzi                                                 |
