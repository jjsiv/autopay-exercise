# Architecture

This repository contains 4 root directories:

- `bootstrap/`, which contains contains resources and configurations related to the cluster bootstrap/setup
- `topics/`, which contains the developer self-service solution for managing Kafka topics
- `tests/`, which contains various example resources for testing purposes
- `example-app/`, which contains an example consumer/producer application for testing purposes

## Cluster setup

The cluster is created locally using minikube and VirtualBox provider:

```bash
minikube start \
    --driver=virtualbox \
    --nodes 3 \
    --container-runtime containerd \
    --cni cilium \
    --cpus 2 \
    --memory 6g \
    --disk-size 20g
```

Once cluster is ready, the next step is to install ArgoCD which will handle everything else.

## GitOps / ArgoCD

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

This will deploy the AppProjects, Applications and ApplicationSets which will have ArgoCD manage the installation and management of itself and everything else. It might take several minutes before everything is up.

### Running UI/CLI

As ArgoCD is not exposed to outside traffic, the UI can be accessed only via port forwarding:

```bash
kubectl port-forward -n argocd svc/argocd-server 8080:80
```

This will let us access ArgoCD on `http://localhost:8080`. Admin credentials can be extracted from the `argocd-initial-admin-secret` secret in `argocd` namespace by running:

```bash
kubectl get secret -n argocd argocd-initial-admin-secret -o yaml | yq .data.password | base64 -d -
```

`argocd` CLI can be used after running:

```bash
kubectl config set-context --current --namespace=argocd
```

This assumes the current kubeconfig context is already set to the ArgoCD cluster.

## Cluster addons

All cluster addons are contained within subdirectories under `bootstrap/addons` and are managed by ArgoCD.

Addon deployments are divided by deployment types: **kustomize** and **helm**. Information about directory structure can be found in their respective directories:

- [Kustomize addons](./bootstrap/addons/kustomize/README.md)
- [Helm addons](./bootstrap/addons/helm/README.md)

For PoC purposes, just two ApplicationSets are used to deploy these addons:
[helm-addons](./bootstrap/argocd/extra/appsets/helm-addons.yaml) and [kustomize-addons](./bootstrap/argocd/extra/appsets/kustomize-addons.yaml).
They both run a single [Git generator](https://argo-cd.readthedocs.io/en/latest/operator-manual/applicationset/Generators-Git/) which generates and templates addon Applications per file or directory found.

## Verification app

A verification app has been created that produces and consumes logs from a Kafka topic. The source code and deployment files are available under `example-app`.
It is deployed to example-app namespace as two depyloments - publisher and subscriber.

It is likewise deployed via ArgoCD, using the [example-app Application](./bootstrap/argocd/extra/apps/example-app.yaml)

# Solution documentation

Two solutions have been created:

- one utilizing Crossplane KafkaTopicClaim XR and Kyverno for validation
- one utiziling only Helm

Both solutions are described below in detail with a step-by-step usage guides for tenants/developers.

## Original solution (KafkaTopicClaim XR, Kyverno)

The exercise required:

- creation of KafkaTopicClaim Crossplane composite resource or similar that creates a KafkaTopic resource
- validation of `retention.ms` topic parameter using Kyverno

An XRD `KafkaTopicClaim` and a matching Composition have been created to solve this. These resources can be found under `bootstrap/addons/kustomize/crossplane-resources/base`.

The XRD requires _topicName_ and _partitions_ to be specified and allows configuration of replication factor and retention time through _replicas_ and _retention_ parameters.
The replication factor is by default set to 3 in the composition.

The composition uses `patch-and-transform` function as it is sufficient for the purpose of this PoC and does not introduce unnecessary complexity.
The Strimzi Kafka cluster label (`strimzi.io/cluster`) is hard coded to the value of `poc-kafka` for the purposes of this PoC.

Additionally, a custom healthcheck in ArgoCD has been configured for KafkaTopicClaim resources. The healthcheck lets ArgoCD understand when the resource is healthy and set the status appropriately.
This in turns allows the developer to view the status of the resource from the UI/CLI or introduce deployment dependencies (in some use cases).
[It can be found in the Helm values file for ArgoCD](./bootstrap/argocd/values.yaml), under `resource.customizations.health.poc.io_KafkaTopicClaim` key.

KafkaTopicClaim manifests for this solution are contained within `topics/solution-1/tenants` subdirectories and are deployed by a `kafka-topics-solution-1` ArgoCD ApplicationSet, which can be found [here](./bootstrap/argocd/extra/appsets/kafka-topics-solution-1.yaml).
The ApplicationSet generates an Application for each directory found in `topics/solution-1/tenants`. This tenant split is arbitrary for PoC purposes.

### Validation with Kyverno

Resources are validated using Kyverno's `ClusterPolicy`. The manifest for this policy can be found [here](./bootstrap/addons/helm/kyverno/extra/policies/validate-kafka-topic-retention.yaml).

The policy will deny KafkaTopicClaims with retention set to a value outside of the 3600000-604800000 (1 hours - 7 days) range.

Example manifests for testing are provided in `tests/` directory. The tests can be performed on a live cluster using `kubectl` or offline using `kyverno` CLI.

- `kubectl apply -f tests/`

```
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

- `kyverno apply bootstrap/addons/helm/kyverno/extra/policies/validate-kafka-topic-retention.yaml --resource tests/`

```bash
Applying 1 policy rule(s) to 4 resource(s)...
policy validate -> resource kafka-topics/KafkaTopicClaim/test-topic failed:
1 - validate-kafka-topic-retention validation error: Retention for the topic must not be lower than 3600000 (1 hour) or higher than 604800000 (7 days). rule validate-kafka-topic-retention failed at path /spec/retention/

policy validate -> resource kafka-topics/KafkaTopicClaim/test-topic failed:
1 - validate-kafka-topic-retention validation error: Retention for the topic must not be lower than 3600000 (1 hour) or higher than 604800000 (7 days). rule validate-kafka-topic-retention failed at path /spec/retention/

policy validate -> resource kafka-topics/KafkaTopicClaim/test-topic failed:
1 - validate-kafka-topic-retention validation error: Retention for the topic must not be lower than 3600000 (1 hour) or higher than 604800000 (7 days). rule validate-kafka-topic-retention failed at path /spec/retention/


pass: 1, fail: 3, warn: 0, error: 0, skip: 0

```

### Limitations

#### Namespacing

There exists a limitation in the TopicOperator component of Strimzi, where only a single namespace can be watched for new KafkaTopic resources. Also described in this [GitHub issue](https://github.com/strimzi/strimzi-kafka-operator/issues/1206).
Paired with Crossplane's ability to only deploy composed resources to the same namespace as the original XR (also described [here](https://github.com/crossplane/crossplane/issues/6759)), this can potentially introduce issues in multi-tenant scenarios,
where at least for visibility purposes, tenants might want/need view access into their KafkaTopicClaim resources in the cluster.

For the purposes of this PoC, TopicOperator has been configured to watch `kafka-topics` namespace for new KafkaTopics and likewise KafkaTopicClaims are deployed to this namespace.

Two workarounds can be considered for the namespace limitation:

- using Crossplane provider `provider-kubernetes`, which allows creation of arbitrary resources across namespaces
- implementing a custom Crossplane provider for Kafka resources rather than an XR
- limiting direct access to the `kafka-topics` namespace for developers and instead abstracting it only through individual ArgoCD Applications

#### Resource validation

Resource validation at CI/local level might be difficult due to the nature of Crossplane. Resources can be rendered locally via `crossplane render`, but this requires at least a working Docker environment and
access to download Crossplane functions.

Additionally, `crossplane render` does not perform validation on the Composite resources and will render even from invalid resources.
For example, we can render `tests/ktc-invalid.yaml`, which is missing required `topicName` field and specifies an unknown `unknownField` field:

```bash
crossplane render \
    tests/ktc-invalid.yaml \
    bootstrap/addons/kustomize/crossplane-resources/base/ktc-composition.yaml \
    bootstrap/addons/kustomize/crossplane-resources/base/functions.yaml \
    --xrd=bootstrap/addons/kustomize/crossplane-resources/base/ktc-xrd.yaml
```

While we can validate our resource using `crossplane beta validate` command, this requires local access to XRD schemas.

Additionally, any admission rules can only be validated using `kyverno` CLI, which requires the developer to have access to both the CLI and policy schemas.

In a pull request based self-service model where developers are expected to commit entire Kubernetes resources (especially considering concerns described in the Namespacing section),
this introduces a large responsibility on developers/approvers/CI system to prevent invalid resources from being committed. Admittedly, most of these issues are resolved if the
self-service process is abstracted through a platform such as Backstage.

### Usage guide

Kafka topics are managed through KafkaTopicClaim resources found in `topics/solution-1/tenants/<application>` directory in this repository.

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

In order to request a new topic, you should raise a Pull Request with a new KafkaTopicClaim resource added in `topics/solution-1/tenants/<application>`, where `application` is your application name, for example `app-1`.
Each application directory can contain multiple KafkaTopicClaim resources. If no directory for your application exists, feel free to create it.

Begin by [forking](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/working-with-forks/fork-a-repo) this repository.
Once done, clone your forked repository locally and create a new KafkaTopicClaim manifest in `topics/solution-1/tenants/<application>` directory.
You can use the example manifest above as a base and modify it to your needs.

Keep in mind that:

- the filename should be unique within directory - make sure you're not overwriting another file
- name of your KafkaTopicClaim as well as the topicName should be unique
- the parameter values meet the requirements described above (in Usage guide), as the request will otherwise be rejected

Once you've created the manifest, you should commit and push it to your fork and raise a pull request to this repository.
If your change meets all the rules, it will be briefly approved and merged.

#### Modify an existing topic

The process is similar to the one for requesting a new topic, except that you don't have to create any new resources.

If you know the topic name, but not the filename, you can locate the file by running:

```bash
grep -Pr 'topicName: <topic name>$' topics/solution-1/tenants
```

Make sure that the changes you're making meet the requirements described in Usage guide, otherwise the pull request will be rejected.

#### Validate your topic

You can validate your KafkaTopicClaim locally before making a pull request by using `crossplane` CLI (for example `app-1/app-1-default-topic.yaml`):

```bash
crossplane beta validate bootstrap/addons/kustomize/crossplane-resources/base/ topics/solution-1/tenants/app-1/app-1-default-topic.yaml
```

You can download the crossplane CLI [here](https://docs.crossplane.io/latest/cli/).

#### Verifying the status of your topic

Once the pull request is merged, you can verify the status of your topic via kubectl or in ArgoCD UI, in `kafka-topics-<application>` Application.
For example, for path `topics/solution-1/tenants/app-1`, the Application will be `kafka-topics-app-1`.

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

Naviate to the ArgoCD UI and the `kafka-topics-<application>` Application. From the resource dashboard, you can search your KafkaTopicClaim and see its status.

The resource health will be marked as `Healthy` once the topic has been configured, `Progressing` if it is in the process of being provisioned, or `Degraded` in case of issues.
By clicking on the resource and the event tab, you can view the events which might provide valuable information.

## Alternative solution (Helm)

The alternative solution to the Crossplane/Kyverno setup utilizes a simple custom Helm chart in order to abstract management of Kafka topics.

The Helm chart can be found in `topics/solution-2/helm-chart`.
It abstracts deployment of KafkaTopics through a simple values file and a `values.schema.json` file used for validation.
The default `values.yaml` does not create any topics by default, rather, it serves as documentation for available options.

Configuration of the KafkaTopic is abstracted in a very similar way to what was done for Crossplane KafkaTopicClaim, where:

- topic names are checked to be valid against actual Kafka naming requirements
- `partitions` field is required
- `replicas` and `retention` are configurable within allowed ranges, with some default values provided

Self service in this solution is achieved through deployment of a Helm chart for each tenant, rather than raw manifests.
The tenant structure can be found in `topics/solution-2/tenants`, where each directory is an arbitrary tenant/application containing a single `topics.yaml` file acting as values for the chart.

An ArgoCD ApplicationSet `kafka-topics-solution-2` is created, which generates individual tenant Applications per directory found in `topics/solution-2/tenants`.
For PoC purposes, the Helm chart is not released to any registry and is instead sourced directly from this repository.
The ApplicationSet can be found [here](./bootstrap/argocd/extra/appsets/kafka-topics-solution-2.yaml).

There are several advantages to this approach:

- Helm tends to be simpler than Crossplane for less complex scenarios
- validation can be performed offline and configuration errors can be prevented before even touching Kubernetes; easy to validate locally or in CI
- easier to perform change preview
- no additional custom healthcheck logic required in tools like ArgoCD to properly detect resource health

On the other hand, it is important to keep in mind that naturally **Helm is not a replacement for neither Crossplane nor Kyverno**.
The only purpose of this alternative solution is to demonstrate a different approach to this specific scenario.

### Validation using values.schema.json

Documentation about schema files can be found in the [Helm documentation](https://helm.sh/docs/topics/charts/#schema-files).

Example values files can be found in `ci/` directory. These can be used for testing against the values schema, for example:

```bash
$ helm template topics/solution-2/helm-chart -f topics/solution-2/helm-chart/ci/missing-partitions.yaml

Error: values don't meet the specifications of the schema(s) in the following chart(s):
kafka-topics:
- at '/topics/my-another_topic': missing property 'partitions'

$ helm template topics/solution-2/helm-chart -f topics/solution-2/helm-chart/ci/retention-out-of-range.yaml

Error: values don't meet the specifications of the schema(s) in the following chart(s):
kafka-topics:
- at '/topics/my-topic/retention': maximum: got 1×10¹², want 6.048×10⁰⁸

```

### Usage guide

Kafka topics are managed through `topics.yaml` files located in `topics/solution-2/tenants/<application>` directories in this repository.
New topics/modifications can be requested through pull requests.

If no topics are defined for the application, the default `topics.yaml` might look like this:

```yaml
topics: {}
```

If the file doesn't exist at all, feel free to create it. Here are all the configuration options:

```yaml
topics:
  my-topic:
    retention: 36400000
    partitions: 3
    replicas: 3
```

Each entry under `topics` is a separate Kafka topic and so each key must be a valid topic name. The possible configuration options are:

- **retention**, which defines log retention time in miliseconds. It must be within range 3600000-604800000. It is optional and 3600000 is the default value.
- **partitions**, which defines number of partitions for this topic. This parameter is required.
- **replicas**, which defines the replication factor. It must be within 1-32767 range. It is optional and 1 is the default value.

#### Request a new topic / modify existing

In order to request a new topic, you should raise a Pull Request modifying the `topics.yaml` file in `topics/solution-1/tenants/<application>`, where `application` is your application name, for example `app-1`.
Each application directory can contain multiple KafkaTopicClaim resources. If no directory for your application exists, feel free to create it.

Begin by [forking](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/working-with-forks/fork-a-repo) this repository.
Once done, edit the desired `topics.yaml` file and add a new topic.
For example, to create a topic `foo-bar` with replication factor set to 3, 5 partitions and a retention time of 4 days, the configuration will look like this:

```yaml
topics:
  foo-bar:
    replicas: 3
    partitions: 5
    retention: 345600000
```

For modifications to existing topics, you can simply search the topic name in the file.

Keep in mind that:

- name of your topic must be unique
- the parameter values meet the requirements described above (in Usage guide), as the request will otherwise be rejected

Once done, commit your changes and raise a pull request.

#### Validate your topic

You can validate as well as preview the resources that will be created with your `topics.yaml` file locally with `helm` CLI:

```bash
helm template topics/solution-2/helm-chart -f topics/solution-2/tenants/<application>/topics.yaml
```

In case your `topics.yaml` doesn't meet requirements, you will receive an error.

[See here for instructions on installing Helm.](https://helm.sh/docs/intro/install/)

#### Verifying the status of your topic

Once the pull request is merged, you can verify the status of your topic via in ArgoCD UI, in `kafka-topics-<application>` Application.
Naviate to the ArgoCD UI and the `kafka-topics-<application>` Application. From the resource dashboard, you can see all KafkaTopic resources for the application.

The resource health will be marked as `Healthy` once the topic has been configured, `Progressing` if it is in the process of being provisioned, or `Degraded` in case of issues.
By clicking on the resource and the event tab, you can view the events which might provide valuable information.
