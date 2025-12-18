# Kustomize addons

This directory contains addons deployed using Kustomize.
The ArgoCD `kustomize-addons` ApplicationSet will generate an Application for each directory contained here with name equal to the directory name.

Each directory must contain a `base` directory with a `kustomization.yaml` file.
Refer to [Kustomize documentation](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/) for more information.

Since we run only a single cluster and environment, we use a simple base setup with no overlays or components, making the setup not much different from simply deploying raw manifests.
