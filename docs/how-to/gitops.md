# Commit SmartHub Configuration to Source Control

This page supports the **Commit to Source Control (GitOps)** tile on the Octant install **Next Steps** screen.

Use it when Octant has generated a SmartHub manifest bundle and you want to save that configuration in GitHub or another source-control system.

## What This Tile Does

On the Next Steps screen, the GitOps tile has two actions:

- **Download Manifests (.zip)** saves the generated SmartHub and solution manifests for the connection you just configured.
- **View GitOps Docs** opens this guide so you can review and commit those manifests to your deployment repository.

In a connected Octant environment, this action downloads the ready-to-review solution bundle that should be committed to your source-control repository.

## What Goes in the Zip

The bundle should contain the manifests needed for the solution, such as:

- Argo CD `Application` manifests.
- SmartHub `MdaiHub` and connection resources.
- OpenTelemetry collector resources.
- Observer and validation resources.
- Destination Secret templates or secret references.
- Additional RBAC or access resources required by the solution.

Octant generates the solution from the UI flow and packages the related manifests into the downloaded `.zip`.

## Before You Commit

Review the downloaded bundle for environment-specific details:

| Area | What to check |
| --- | --- |
| Cluster target | Namespace, cluster name, target environment, and repository path. |
| Chart version | `mdai-hub` target revision and any values files referenced by the application. |
| Values | Production overrides, storage classes, PV/PVC settings, retention, and cleanup behavior. |
| Secrets | Secret names and references. Do not commit literal API keys, account tokens, or vendor credentials. |
| Telemetry | Enabled logs, metrics, traces, destinations, sampling, filtering, routing, observer, and validator resources. |
| Deployment metadata | If the bundle includes Argo CD or another deployment-controller resource, review its destination, sync behavior, pruning behavior, retry settings, and drift-handling settings. |

Commit sealed, encrypted, or externally referenced secret material only when it matches the organization-approved process.

## Unzip the Bundle

After clicking **Download Manifests (.zip)**, the file is usually saved to your Downloads folder. Unzip it before copying the solution into your GitOps repository.

```shell
cd ~/Downloads
unzip smarthub-manifests.zip -d smarthub-manifests
```

If your browser saved the file with the connection name, use that filename instead:

```shell
cd ~/Downloads
unzip <connection-name>-manifests.zip -d <connection-name>-manifests
```

Open the unzipped folder and confirm it contains the manifests, values, and resource files you expect before committing them.

## Commit the Bundle

Use the repository that owns the reviewed SmartHub solution configuration for the target environment.

```shell
cp -R ~/Downloads/smarthub-manifests/* path/to/deployment-repo/
cd path/to/deployment-repo
git checkout -b deploy/smarthub-production
git add .
git commit -m "Deploy SmartHub production configuration"
git push origin deploy/smarthub-production
```

Open a pull request and request review from the operators responsible for the cluster, storage, telemetry destination, and security posture.

## Connect Source Control to Deployment

Committing the bundle saves the generated SmartHub configuration to source control. Deployment still follows the process your team uses for the target environment.

If Argo CD owns the cluster desired state, confirm that Argo CD is configured to read the repository path where you committed the bundle. The Octant download does not automatically connect Argo CD to that repository path.

After the pull request is approved and merged:

1. Confirm the deployment controller can read the repository, branch, and path.
2. Confirm required secrets exist in the target namespace or secret manager.
3. Sync the deployment application or wait for automated reconciliation.
4. Confirm the deployed application is healthy and synced.
5. Validate SmartHub readiness, connection status, Data Fidelity Validation, and Clarity data.

The source-controlled bundle may include an Argo CD application, `mdai-hub` Helm application values, SmartHub variables, collectors, observer resources, validator resources, destination secret material, and supporting RBAC. The exact files depend on the Octant UI flow and telemetry options selected before download.

A future Octant release may add a GitHub connection that can save generated solutions directly to source control and use that connection as part of a centralized policy-governance workflow for Octant solutions. Until then, treat this guide as the source-control handoff: Octant generates the bundle, your repository stores the reviewed configuration, and your deployment process applies it.

## Saving Later Changes

When a connection or setting changes in Octant and you want source control to reflect the new configuration:

1. Download a fresh manifest bundle.
2. Compare it against the deployment repository.
3. Preserve intentional environment-specific overrides.
4. Open a reviewed pull request.
5. Apply the merged change through the environment's normal deployment process.
6. Validate telemetry flow and rollback through the same source-control process if validation fails.

## Related Pages

- [Deploy SmartHub to Production](production.md)
- [Setup and Operations](../setup.md)
- [Connections and Integrations](../connections.md)
- [API Reference](../api.md)
