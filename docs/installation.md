# Installation

- [From Local Build](#local-deployment-notes) - Run as a local deployment.
- [Via Argo CD](https://github.com/MyDecisive/octant-argo-example/blob/main/docs/installation.md) - follow our detailed Argo CD Example for running Octant in a cluster with Argo CD. Note: This will take you to a different repo, we suggest opening this in a new tab.

## Local Deployment Notes

Octant runs in Kubernetes and uses its service account, Role, and RoleBinding to manage the resources it needs. The Helm chart lives in `deployment/`.

Build the container image:

```shell
make docker-build
```

Package the Helm chart:

```shell
make helm-package
```

Install or upgrade the chart:

```shell
helm upgrade octant ./octant-0.x.x.tgz --install
```
