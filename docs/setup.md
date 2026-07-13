# Octant Setup and Operations

Use this guide to choose an Octant environment, install Octant, move configuration through review, validate the deployment, and roll back changes when needed.

To skip ahead to installation, select your installation method
1. [Argo CD Install](#argo-cd-install-path)
2. [Helm Install](#direct-helm-install-path)

## Environment Choices

| Area | Development | Shared or production-oriented |
| --- | --- | --- |
| Purpose | Local evaluation, demos, workflow testing, and early integration validation. | Shared, persistent, or customer-facing telemetry operations. |
| Cluster | Usually local Kind from `octant-argo-example`, or another disposable Kubernetes cluster. | Organization-managed Kubernetes cluster. |
| Deployment | Helper commands, local App-of-Apps bootstrap, or direct Helm install. | Reviewed source-controlled deployment flow or approved direct Helm install. |
| Credentials | Local test credentials and short-lived tokens. | Managed secrets and approved access controls. |
| Data | Mock, demo, or disposable telemetry. | Real service telemetry subject to operational controls. |
| Change control | Lightweight local iteration. | Pull request, review, approval, deployment, validation, and rollback. |

Use development environments to learn Octant workflows, test SmartHub installation, try Argo CD integration, run direct Helm installation into a reachable cluster, generate mock telemetry with `octant-demo-load`, and validate connection or Clarity behavior before using real workloads.

Use shared or production-oriented environments to operate shared telemetry pipelines, manage source-controlled SmartHub and Octant configuration, route real telemetry to approved destinations, and run repeatable validation and cost review workflows.

Treat the install **Next Steps** screen as a docs entry point after an Octant install workflow: use **Deploy to Production** for production readiness and **Commit to Source Control (GitOps)** for manifest handoff.

## Prerequisites

You'll need the following tools to run Octant

1. [Docker](https://www.docker.com/products/docker-desktop/)
1. [Kubernetes](https://kubernetes.io/releases/download/)
1. [`kubectl` and `kind`](https://kubernetes.io/docs/tasks/tools/)
1. [Helm](https://helm.sh/docs/intro/install/)
1. [`just`](https://just.systems/man/en/installation.html)
1. [Argo CD CLI](https://argo-cd.readthedocs.io/en/stable/getting_started/#2-download-argo-cd-cli)
1. [`k9s`](https://k9scli.io/topics/install/) (optional)


Confirm these basics before any install path:

- A Kubernetes cluster is available.
- `kubectl` can reach the target context.
- Helm is available for chart packaging or installation workflows.
- The cluster can pull required container images.
- The cluster can reach telemetry destinations such as Datadog when configured.
- Operators can reach Octant through a service, ingress, or port-forward appropriate for the environment.
- DNS, ingress, TLS, and allow-list requirements are known before a shared or production-oriented install starts.

Before installing Octant outside a development environment, also confirm permission to:

- Read and write the source-controlled deployment repository.
- Create or update the namespaces used by Octant, SmartHub, Argo CD, and telemetry destinations.
- Manage Kubernetes Secrets or approved secret references.
- Configure Argo CD applications or the approved deployment controller.
- Install or upgrade Helm releases directly when using the Helm path.
- Validate pods, services, events, logs, and application health after deployment.

## SmartHub Requirement

MyDecisive SmartHub powers Octant. SmartHub prerequisites come from the `mdai-hub` chart and currently include Kubernetes `1.24+`, Helm `3.9+`, and optional cert-manager behavior depending on chart values.

If SmartHub is not already installed, use the approved SmartHub installation path before relying on Octant workflows that require SmartHub runtime services. For SmartHub-level installation and chart behavior, use `https://github.com/MyDecisive/mdai-hub/blob/main/docs/quickstart.md` as the source of truth.

## Argo CD Install Path

Use this path when the target cluster is managed through GitOps and Argo CD should reconcile Octant and SmartHub from reviewed desired state.

In a development environment, use `https://github.com/MyDecisive/octant-argo-example` to bootstrap local Argo CD and the Octant App-of-Apps workflow:

```shell
git clone https://github.com/MyDecisive/octant-argo-example.git
cd octant-argo-example
just prereqs
just octant-bootstrap
just port-forward-octant
```

Open:

```text
http://localhost:5678/
```

When the Octant UI asks for Argo CD connection details, run:

```shell
just local-setup
```

Use the generated connection name, Argo CD cluster URL, and Argo CD API token only for the development environment that generated them.

For shared or production-oriented environments, treat Argo CD configuration as source-controlled deployment configuration. Do not copy development credentials or local Kind assumptions into a shared environment.

## Direct Helm Install Path

Use this path when an operator should install or upgrade Octant directly into a reachable Kubernetes cluster with Helm. This path is useful for development clusters, proof-of-concept environments, or approved shared environments where GitOps is not the deployment controller.

Install or upgrade into the target cluster and namespace:

```shell
kubectl config current-context
kubectl create namespace octant --dry-run=client -o yaml | kubectl apply -f -
helm install octant oci://ghcr.io/mydecisive/charts/octant --version 0.x.x
```

See all [available tags](https://github.com/MyDecisive/octant/tags)

When using this path for a shared or production-oriented environment, keep the exact chart package, values file, namespace, image tag, and credential references under review and source control.

If SmartHub is not already installed, install it with the approved `mdai-hub` Helm chart or the published SmartHub installation flow before relying on Octant workflows that require SmartHub runtime services.

## Source Control and Review

Commit deployment and configuration artifacts that define desired state, including:

- SmartHub and Octant installation configuration.
- Argo CD application manifests.
- Connection manifests generated for deployment.
- Destination and integration references that belong in source control.
- Non-secret configuration for telemetry signals, routing, and sampling policy.

Do not commit literal API keys, account tokens, or local development credentials.

Review changes before deployment for namespace and cluster target, SmartHub chart or image changes, Argo CD application source and branch, destination endpoints, credential references, enabled telemetry signals, and sampling or filtering changes that affect volume or cost.

For the full manifest handoff flow, see [Commit SmartHub Configuration to Source Control](how-to/gitops.md). For production storage, PV, PVC, and rollout checks, see [Deploy SmartHub to Production](how-to/production.md).

## Development to Shared Checklist

Use this checklist when moving from a development proof path to a shared or production-oriented environment:

- Replace Kind, localhost, and port-forward assumptions with the target cluster access pattern.
- Replace local helper credentials with managed secret references.
- Confirm chart versions, namespaces, image pull access, and network access.
- Commit generated manifests or configuration to the deployment repository.
- Apply through Argo CD or perform an approved direct Helm install.
- Validate Octant reachability, SmartHub readiness, connection status, Data Fidelity Validation, and Clarity data.

## Validation

After installation:

```shell
kubectl get pods --all-namespaces
kubectl get svc --all-namespaces
```

For the development helper, use:

```shell
just octant-status
just argocd-apps
```

Expected result: Octant is reachable, Argo CD applications or Helm releases are healthy, SmartHub resources are ready, and connection setup can proceed.

## Rollback and Teardown

Rollback should use the same control path that applied the change:

1. Revert the source-control change.
2. Let Argo CD or the approved deployment system reconcile the prior state, or run the reviewed Helm rollback/uninstall path used by the direct install workflow.
3. Watch the affected resources become healthy.
4. Run Data Fidelity Validation and check Clarity for unexpected volume changes.

For development teardown:

```shell
just cleanup
```

For shared-environment teardown or migration, remove or revert the source-controlled deployment configuration, allow Argo CD or the deployment system to reconcile the change, or run the approved Helm uninstall path and confirm that Kubernetes resources are deleted or moved as expected.

## Related Pages

- [Deploy SmartHub to Production](how-to/production.md)
- [Commit SmartHub Configuration to Source Control](how-to/gitops.md)
- [Connections and Integrations](connections.md)
- [Telemetry Insights](telemetry.md)
- [Architecture](architecture.md)
