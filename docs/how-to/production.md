# Deploy SmartHub to Production

This page supports the **Deploy to Production** tile on the Octant install **Next Steps** screen.

Use it after an Octant install workflow has completed and you are ready to prepare a real SmartHub rollout. Production deployment requires reviewed cluster access, values, secrets, storage, and GitOps configuration.

## What This Tile Means

The tile is a handoff from a successful test workflow to a production-readiness review. It does not mean the test configuration should be copied directly into production.

Before you deploy, decide:

- Which production cluster and namespace will run SmartHub.
- Which `mdai-hub` chart version will be used.
- Whether Argo CD or another approved deployment controller will own the release.
- Which values files and generated manifests will be committed to the deployment repository.
- Which storage classes, PersistentVolumes, PersistentVolumeClaims, retention settings, and cleanup rules apply.
- Which secret-management process will provide destination credentials and platform passwords.

## Production Readiness Checklist

Confirm these items before the first production sync:

| Area | What to confirm |
| --- | --- |
| Cluster target | Kubernetes context, namespace, ingress, DNS, TLS, and allow-list requirements are approved. |
| Chart compatibility | Octant, generated manifests, and the selected `mdai-hub` chart version are reviewed together. |
| Deployment control | Argo CD or the approved controller reconciles source-controlled desired state. |
| Secrets | Secrets are managed through the approved process, not copied from a local test environment. |
| Storage | NATS JetStream, Prometheus, Valkey, GreptimeDB, and Tracealyzer storage behavior is reviewed. |
| Validation | Operators know how to verify pods, services, PVCs, SmartHub readiness, connection status, Data Fidelity Validation, and Clarity. |
| Rollback | The rollback path and PVC/data-retention expectations are known before production changes are merged. |

## Review the SmartHub Values

Start with the version-pinned `mdai-hub` values for the version you plan to deploy. For chart version `0.10.0`, review:

- [`values.yaml`](https://github.com/MyDecisive/mdai-hub/blob/v0.10.0/values.yaml)
- [`tracealyzer-values.yaml`](https://github.com/MyDecisive/mdai-hub/blob/v0.10.0/tracealyzer-values.yaml)
- [`greptimedb-values.yaml`](https://github.com/MyDecisive/mdai-hub/blob/v0.10.0/greptimedb-values.yaml)

Keep production overrides in environment-specific values files in the deployment repository.

## Storage Settings to Review

Production storage is the main difference between a quick test install and a durable cluster install.

| Component | Production guidance |
| --- | --- |
| NATS PVs | Decide whether `persistentStorage.nats.pv.create` should create PVs or whether the cluster team will pre-provision them. |
| NATS JetStream PVCs | Enable and size `nats.config.jetstream.fileStore.pvc` for the production storage class. Keep memory storage disabled for durable production use. |
| Prometheus | Set retention, storage, remote write, and remote read according to the observability retention plan. |
| Valkey | Confirm persistence, sizing, and retention needs for runtime state. |
| GreptimeDB | If `greptimedb-standalone` is enabled, configure persistence, object storage, authentication, and cloud service account annotations. |
| Tracealyzer | If `mdai-tracealyzer` is enabled, review replica count, Valkey and GreptimeDB secrets, migration job behavior, resources, and topology requirements. |

Example production overlay:

```yaml
persistentStorage:
  nats:
    pv:
      create: true
      platform: aws
      storageClass: gp2
      size: 20Gi

nats:
  config:
    jetstream:
      memStorage:
        enabled: false
      fileStore:
        enabled: true
        pvc:
          enabled: true
          storageClassName: gp2
          size: 20Gi

greptimedb-standalone:
  enabled: true
  persistence:
    storageClass: gp2
    size: 50Gi
```

Adjust storage classes, sizes, zones, cloud volume IDs, buckets, regions, and service account annotations for the actual production cluster.

## Recommended Flow

1. Finish the Octant install workflow and review the **Next Steps** screen.
2. Use **Download Manifests (.zip)** from the GitOps tile to save the generated bundle.
3. Add production values and environment-specific overrides.
4. Commit the manifest bundle and values to the production deployment repository.
5. Open a pull request for platform, storage, observability, and security review.
6. Merge and let Argo CD or the approved controller reconcile.
7. Validate SmartHub readiness, PVC binding, connection status, Data Fidelity Validation, and Clarity data.

## Rollback

Rollback should follow the same path that deployed the change:

1. Revert the production repository change.
2. Let Argo CD or the approved controller reconcile the previous desired state.
3. Confirm whether PVCs and retained telemetry data should remain in place.
4. Validate SmartHub readiness and telemetry flow after rollback.

Check chart cleanup behavior before uninstalling production releases. Cleanup hooks that delete PVCs may conflict with retention requirements.

## Related Pages

- [Commit SmartHub Configuration to Source Control](gitops.md)
- [Setup and Operations](../setup.md)
- [Connections and Integrations](../connections.md)
- [Architecture](../architecture.md)
