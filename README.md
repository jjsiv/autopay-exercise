# Architektura

## Cluster

Cluster został stworzony lokalnie przy użyciu minikube i VirtualBoxa:

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

## Bootstrap

Wszystko na clusterze jest deployowane i managowane przez ArgoCD (w tym samo ArgoCD). Wymaga to jednak pierwotnego ręcznego zainstalowania ArgoCD i stworzenia aplikacji (Application) i projektu (AppProject), które już potem będą automatycznie deployować manifesty z konkretnych directory.

Kroki do ręcznego zdeployowania ArgoCD:

```bash
kubectl create namespace argocd
helm template --release-name argocd --repo https://argoproj.github.io/argo-helm argo-cd --version 9.1.* --values bootstrap/argocd/values.yaml --namespace argocd | kubectl create -f -
kubectl create -f bootstrap/argocd/extra -R
```

Do instalacji używamy `helm template | kubectl create -f -` dlatego, że mniej więcej w ten sam sposób ArgoCD działa z Helmem, w kontraście do chociażby Fluxa.
Chcemy więc uniknąć potencjalnego konfliktu między Helmem a ArgoCD.
