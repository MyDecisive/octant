# Octant Documentation

Octant helps you install SmartHub, connect telemetry sources, validate delivery, and use Clarity to understand telemetry volume, cost, and health.

## Start Here

| What you want to do | Go to |
| --- | --- |
| Install Octant, choose an environment, validate setup, or roll back a change. | [Setup and Operations](setup.md) |
| Browse task-focused guides from Octant UI workflows. | [How-To Guides](how-to/index.md) |
| Use the **Deploy to Production** tile on the install Next Steps screen. | [Deploy SmartHub to Production](how-to/production.md) |
| Use the **Commit to Source Control (GitOps)** tile and downloaded manifest `.zip`. | [Commit SmartHub Configuration to Source Control](how-to/gitops.md) |
| Configure Argo CD or Datadog, create a connection, reroute telemetry, or troubleshoot missing data. | [Connections and Integrations](connections.md) |
| Review Clarity, Top Talkers, sampling, cost, volume, time ranges, or missing services. | [Telemetry Insights](telemetry.md) |
| Understand how Octant and SmartHub fit together. | [Architecture](architecture.md) |
| Look up Octant API services and operation paths. | [API Reference](api.md) |
| Build and run Octant locally. | [Development](development.md) |

## Key Terms

For detailed setup and operational definitions, use the task pages linked above.

- SmartHub installation and environment choices: [Setup and Operations](setup.md).
- Connections, integrations, destinations, and Data Fidelity Validation: [Connections and Integrations](connections.md).
- Clarity, Top Talkers, sampling, volume, cost, and missing services: [Telemetry Insights](telemetry.md).

## Common Workflow

1. Choose a development, shared, or production-oriented environment.
2. Install Octant and SmartHub.
3. Save the required Argo CD or Datadog integrations.
4. Create a connection with the metrics, logs, or traces you want to manage.
5. Reroute telemetry through the SmartHub connection.
6. Run Data Fidelity Validation.
7. On the install **Next Steps** screen, use Clarity to review telemetry, **Deploy to Production** for rollout planning, or **Commit to Source Control (GitOps)** to download and commit manifests.
8. For shared or production environments, reconcile committed manifests through Argo CD or the approved deployment controller.

## Related Pages

- [Setup and Operations](setup.md)
- [How-To Guides](how-to/index.md)
- [Deploy SmartHub to Production](how-to/production.md)
- [Commit SmartHub Configuration to Source Control](how-to/gitops.md)
- [Connections and Integrations](connections.md)
- [Telemetry Insights](telemetry.md)
- [API Reference](api.md)
