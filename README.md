# Setup overview

This repository contains 2 primary directories:

- `bootstrap/`, which contains contains resources and configurations related to the cluster bootstrap/setup
- `topics/`, which contains the developer self-service solution for managing Kafka topics

# Cluster setup

The cluster is created locally using minikube and VirtualBox provider:

```bash
minikube start \
    --driver=virtualbox \
    --nodes 3 \
    --container-runtime containerd \
    --cni cilium \
    --cpus 2 \
    --memory 4g \
    --disk-size 20g
```

Once cluster is ready, the next step is to install ArgoCD which will handle everything else.

## ArgoCD

ArgoCD setup is contained within `bootstrap/argocd` directory.

Initial installation of ArgoCD is performed manually via the following commands:

```bash
kubectl create namespace argocd
helm template --release-name argocd --repo https://argoproj.github.io/argo-helm argo-cd --version 9.1.* --values bootstrap/argocd/values.yaml --namespace argocd | kubectl create -f -
```

We install ArgoCD using `helm template | kubectl create -f -`, as that imitates how ArgoCD itself handles Helm charts (as opposed to Flux which actually runs `helm install`).

Once ArgoCD is running, we deploy the manifests contained in `argocd/bootstrap/extra`:

```bash
kubectl create -f bootstrap/argocd/extra -R
```

This will deploy the AppProjects, Applications and ApplicationSets which will have ArgoCD manage the installation and management of itself as well asl other resources found in this repository.

## Cluster addons

All cluster addons are contained within subdirectories under `bootstrap/addons` and are managed by ArgoCD.

Addon deployments are divided by deployment types: **kustomize** and **helm**. Information about directory structure can be found in their respective directories:

- [Kustomize addons](./bootstrap/addons/kustomize/README.md)
- [Helm addons](./bootstrap/addons/helm/README.md)

For PoC purposes, just two ApplicationSets are used to deploy these addons:
[helm-addons](./bootstrap/argocd/extra/appsets/helm-addons.yaml) and [kustomize-addons](./bootstrap/argocd/extra/appsets/kustomize-addons.yaml).
They both run a single [Git generator](https://argo-cd.readthedocs.io/en/latest/operator-manual/applicationset/Generators-Git/) which generates and templates addon Applications per file or directory found.
