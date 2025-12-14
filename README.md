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

### Running UI/CLI

As ArgoCD is not exposed to outside traffic, the UI can be accessed only via port forwarding:

```bash
kubectl port-forward -n argocd svc/argocd-server 8080:80
```

This will let us access ArgoCD on `http://localhost:8080`. Admin credentials can be extracted from the `argocd-initial-admin-secret` secret in `argocd` namespace.

`argocd` CLI can be used after running:

```bash
argocd login --core
kubectl config set-context --current --namespace=argocd
```

This assumes the current kubectl config is already set to the ArgoCD cluster.

## Cluster addons

All cluster addons are contained within subdirectories under `bootstrap/addons` and are managed by ArgoCD.

Addon deployments are divided by deployment types: **kustomize** and **helm**. Information about directory structure can be found in their respective directories:

- [Kustomize addons](./bootstrap/addons/kustomize/README.md)
- [Helm addons](./bootstrap/addons/helm/README.md)

For PoC purposes, just two ApplicationSets are used to deploy these addons:
[helm-addons](./bootstrap/argocd/extra/appsets/helm-addons.yaml) and [kustomize-addons](./bootstrap/argocd/extra/appsets/kustomize-addons.yaml).
They both run a single [Git generator](https://argo-cd.readthedocs.io/en/latest/operator-manual/applicationset/Generators-Git/) which generates and templates addon Applications per file or directory found.

# Solution documentation

Two solutions have been created:

- one utilizing Crossplane KafkaTopicClaim XR and Kyverno for validation
- one utiziling only Helm

Both solutions are described below in detail with a step-by-step usage guides for tenants/developers.

## Original solution (KafkaTopicClaim XR, Kyverno)

The exercise required:

- creation of KafkaTopicClaim Crossplane composite resource or similar that creates a KafkaTopic resource
- validation of `retention.ms` topic parameter using Kyverno

An XRD `KafkaTopicClaim` and a matching Composition have been created to solve this. These resources can be found under `bootstrap/addons/helm/crossplane/extra/xrs/KafkaTopicClaim`.

The XRD requires _topicName_ and _partitions_ to be specified and allows configuration of replication factor and retention time through _replicas_ and _retention_ parameters.
The replication factor is by default set to 3 in the composition.

The composition uses `patch-and-transform` function as it generally works well for the purpose of this PoC.
It also hard codes the Strimzi Kafka cluster label (`strimzi.io/cluster`) to the value of `poc-kafka` for the purposes of this PoC.

Additionally, a custom healthcheck in ArgoCD has been configured for KafkaTopicClaim resources. The healthcheck lets ArgoCD understand when the resource is healthy and set the status appropriately.
This in turns allows the developer to view the status of the resource from the UI or CLI or, in more complex scenarios, introduce deployment dependencies.
[It can be found in the Helm values file for ArgoCD](./bootstrap/argocd/values.yaml), under `resource.customizations.health.poc.io_KafkaTopicClaim` key.

KafkaTopicClaim manifests for this solution are contained within `topics/solution-1` directory and are deployed by a single `kafka-topics` ArgoCD Application, which can be found [here](./bootstrap/argocd/extra/apps/kafka-topics.yaml).

### Validation with Kyverno

Resources are validated using Kyverno's `ClusterPolicy`. The manifest for this policy can be found [here](./bootstrap/addons/helm/kyverno/extra/policies/validate-kafka-topic-retention.yaml).

The policy will deny KafkaTopicClaims with retention set to a value outside of the 3600000-604800000 (1 hours - 7 days) range.

This can be tested by deploying the example manifests found under `tests/`. We can observe the behaviour as follows (the output has been cleaned up for brevity):

```
$ kubectl apply -f tests/

kafkatopicclaim.poc.io/test-topic configured

for: "tests/ktc-retention-too-high.yaml": error when patching "tests/ktc-retention-too-high.yaml": admission webhook "validate.kyverno.svc-fail" denied the request:
resource KafkaTopicClaim/kafka-topics/test-topic was blocked due to the following policies
validate:
  validate-kafka-topic-retention: 'validation error: Retention for the topic must
    not be lower than 3600000 (1 hour) or higher than 604800000 (7 days). rule validate-kafka-topic-retention
    failed at path /spec/retention/'

for: "tests/ktc-retention-too-low.yaml": error when patching "tests/ktc-retention-too-low.yaml": admission webhook "validate.kyverno.svc-fail" denied the request:
resource KafkaTopicClaim/kafka-topics/test-topic was blocked due to the following policies
validate:
  validate-kafka-topic-retention: 'validation error: Retention for the topic must
    not be lower than 3600000 (1 hour) or higher than 604800000 (7 days). rule validate-kafka-topic-retention
    failed at path /spec/retention/'

```

### Limitations

There exists a limitation in the TopicOperator component of Strimzi, where only a single namespace can be watched for new KafkaTopic resources. Also described in this [GitHub issue](github.com/strimzi/strimzi-kafka-operator/issues/1206).
Paired with Crossplane's ability to only deploy composed resources to the same namespace as the original XR, this can potentially introduce issues in multi-tenant scenarios,
where, at least for visibility purposes, tenants might want/need view access into their KafkaTopicClaim resources in the cluster.
Ultimately, this can but doesn't necessary have to be a problem, depending on whether any secrets are stored in the resources, etc.

For the purposes of this PoC, TopicOperator has been configured to watch `kafka-topics` namespace for new KafkaTopics and likewise KafkaTopicClaims are deployed to this namespace.

#### Possible workarounds

Two workarounds can be considered for the namespace limitation:

- using Crossplane provider `provider-kubernetes`, which allows creation of arbitrary resources across namespaces
- limiting direct access to the `kafka-topics` namespace for developers and instead abstracting it only through individual ArgoCD Applications

### Usage guide

Kafka topics are managed through KafkaTopicClaim resources found in `topics/solution-1` directory in this repository.

Here is an example KafkaTopicClaim with its possible configuration options:

```yaml
apiVersion: poc.io/v1alpha1
kind: KafkaTopicClaim
metadata:
  name: my-topic
spec:
  topicName: my_topic
  retention: 604800000
  replicas: 3
  partitions: 1
```

The parameters `topicName` and `partitions` are **required**. topicName must be unique and a valid Kafka topic name.

Parameter `replicas` can be used to configure the replication factor of the topic. It takes an integer value between 1-32767. Default value is 3.

Parameter `retention` can be used to specify retention time for logs in miliseconds. Minimum allowed value is 3600000 (1h), while maximum is 604800000 (7 days).

#### Request a new topic

In order to request a new topic, you should raise a Pull Request with a new KafkaTopicClaim resource added in `topics/solution-1`.

Begin by [forking](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/working-with-forks/fork-a-repo) this repository.
Once done, clone your forked repository locally and create a new KafkaTopicClaim manifest in `topics/solution-1` directory.
You can use the example manifest above as a base and modify it to your needs.

Keep in mind that:

- the filename should be unique - make sure you're not overwriting another file
- name of your KafkaTopicClaim as well as the topicName should be unique
- the parameter values meet the requirements described above (in Usage guide), as the request will otherwise be rejected

Once you've created the manifest, you should commit and push it to your fork and raise a pull request to this repository.
If your change meets all the rules, it will be briefly approved and merged.

#### Modify an existing topic

The process is similar to the one for requesting a new topic, except that you don't have to create any new resources.

If you know the topic name, but not the filename, you can locate the file by running:

```bash
grep -Pr 'topicName: <topic name>$' topics/solution-1/
```

Make sure that the changes you're making meet the requirements described in Usage guide, otherwise the pull request will be rejected.

#### Verifying the status of your topic

Once the pull request is merged, you can verify the status of your topic via kubectl or in ArgoCD UI, in `kafka-topics` Application.

##### kubectl

With kubectl, you can run this command (replace "resource name" placeholder with the name of the KafkaTopicClaim you changed/created):

```
kubectl get ktc -n kafka-topics <resource name>
```

This should yield output like this:

```
NAME          SYNCED   READY   COMPOSITION       AGE
test-topic    True     False   kafkatopicclaim   4h16m
```

The key of interest is column `READY`. Once it is `True`, you will then know that the Kafka topic has been configured and is ready to use.
In case the status is `False`, you can run:

```bash
kubectl describe ktc -n kafka-topics <resource name>
```

This will let you view more information about the resource, including events, which might provide valuable information.

##### ArgoCD

Naviate to the ArgoCD UI and the kafka-topics Application. From the resource dashboard, you can search your KafkaTopicClaim and see its status.

The resource health will be marked as `Healthy` once the topic has been configured, `Progressing` if it is in the process of being provisioned, or `Degraded` in case of issues.
By clicking on the resource and the event tab, you can view the events which might provide valuable information.

## Alternative solution (Helm)
