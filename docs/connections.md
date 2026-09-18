# Connections and Integrations

Use this guide to configure integrations, create SmartHub connections, set destinations, reroute ingestion, validate telemetry, and troubleshoot missing data.

## Integrations

An integration stores reusable external system configuration. Octant currently uses Argo CD integration details for deployment operations and Datadog integration details for telemetry delivery.

## Argo CD Integration

Argo CD integrations include an integration name, Argo CD API endpoint, and Argo CD account token. In the development workflow, our[`octant-argo-example` repo](https://github.com/MyDecisive/octant-argo-example) can generate local setup values with:

```shell
just local-setup
```

Use those values only for the development environment that generated them.

## Datadog Agent or Datadog Destination Setup

Octant can use Datadog integration information as a telemetry destination. In the development helper, a new Datadog Agent can be installed with Helm and a Kubernetes Secret can hold the API key:

```shell
helm repo add datadog https://helm.datadoghq.com
helm repo update
helm install datadog-agent -f connections/datadog/dd_values.yaml datadog/datadog --create-namespace -n datadog
kubectl -n datadog create secret generic datadog-secret --from-literal api-key=*****dd_api_key*****
```

For shared or production-oriented environments, use the organization's approved Datadog integration and secret-management process instead of copying development credentials.

## SmartHub Connection Creation

A SmartHub connection defines:

- Namespace and connection name.
- Enabled telemetry signals: metrics, logs, traces, or a combination.
- Deployment type and integration name.
- Destinations and destination integration names.

After a connection exists, Octant can list connections, fetch a connection by name, delete a connection and its resources (via API), or generate deployment manifests in JSON or YAML.

When a GitHub App GitOps integration is configured (via Settings in octant-ui, or the `GitOpsService.SaveGitHubAppConnection` RPC), generating manifests also opens a pull request against a configured GitHub CI/CD repository, authenticating as a GitHub App installation — similar to [ArgoCD's GitHub App credential support](https://argo-cd.readthedocs.io/en/stable/user-guide/private-repositories/#github-app-credential). This lets ArgoCD reconcile desired state from a reviewed GitOps repository instead of relying on a manually downloaded and applied zip artifact. For each publish, Octant:

1. Creates or updates a dedicated branch (`<branchPrefix>/<namespace>-<connectionName>`) off the configured base branch, containing the generated manifests under:

   ```text
   <basePath>/<namespace>/<connectionName>/<manifest-file>
   ```
2. Commits the changes to that branch as the configured committer.
3. Opens a pull request from that branch into the base branch, or updates the existing one if Octant already opened one for that connection.

GitHub App integration credentials are stored server-side as a Kubernetes Secret (`mdai-github-app-integration`, one of the octant service account's managed [`integrationSecretNames`](../deployment/values.yaml)) rather than in Helm values or environment variables, so the PEM private key is never held in plaintext configuration. Configure the integration once via octant-ui Settings (App ID, installation ID, PEM private key, repository owner/name, base branch, branch prefix, base path, and committer identity) — the GitHub App installation needs repository `contents:write` and `pull_requests:write` permission for the target repository.

## Validation
## `octant-demo-load` for Mock Telemetry When Service Data Is Unavailable

When service data is unavailable in a development environment, use our [`octant-demo-load` tool](https://github.com/MyDecisive/octant-demo-load) as a demo and load-generation helper. It can emit realistic Datadog or OTLP traces into a supplied ingest endpoint.

Example development pattern:

```shell
export OCTANT_DEMO_API_KEY=<your-dd-key>
octant-demo --context=<development-cluster-context>
```

The demo tool is not required for production use.

<!-- TODO: uncomment when multiple destinations are supported

## Reroute Ingestion Workflow

Use reroute ingestion when telemetry should flow through the SmartHub collectors managed by Octant instead of going directly from workloads to a vendor endpoint.

1. Confirm the target connection and namespace.
2. Confirm enabled telemetry signals.
3. Update workload, agent, or collector configuration to send telemetry to the SmartHub ingress endpoint.
4. Confirm the destination integration is configured.
5. Validate received and sent telemetry metrics. -->

## Destination Setup

Destination setup connects telemetry from SmartHub to a vendor or downstream system. For Datadog, confirm the API URL, API key reference, and selected telemetry signals.

After updating destinations, validate that SmartHub collectors are sending telemetry and that the destination receives the expected logs, metrics, or traces.

Connection and destination changes can affect runtime settings in SmartHub. Treat routing, signal, destination, and sampling changes as controlled updates: apply them through the approved deployment path, then validate that the runtime state and telemetry flow match the intended configuration.

## Access and Permission Inputs

Before creating or changing a connection in a shared or production-oriented environment, confirm:

- The Kubernetes namespace where SmartHub connection resources will be created.
- The deployment integration that can apply the generated configuration.
- The destination integration and secret reference for vendor credentials.
- The telemetry signals allowed for the target environment.
- The operators or reviewers responsible for approving the change.

Do not copy development tokens or local Kind values into a shared environment.

## Connection Status Meanings

Connection status should explain:

- Whether the collector is receiving telemetry.
- Whether the collector is sending telemetry.
- Whether clients are connected for the configured telemetry types.
- Whether telemetry satisfies data integrity checks.
- Per-signal validation results for logs, metrics, and traces.

Octant APIs use in-cluster Prometheus metrics to inform collector, hub, and connection health. Validation components also write metrics that help explain Data Fidelity Validation outcomes.

## Data Verification

Run Data Fidelity Validation after creating or changing a connection. Validation should check whether telemetry is flowing and whether ingress and egress payloads satisfy parity or policy expectations.

For Datadog validation troubleshooting, inspect validation metrics, collector flow metrics, validator logs, and failed correlation IDs. See [Debug Connection Failures](how-to/connection-failures.md).

## Troubleshooting Missing Logs, Traces, or Services

| Symptom | Likely cause | What to check |
| --- | --- | --- |
| No logs or traces | Workload is not sending telemetry to SmartHub. | Reroute ingestion settings, agent configuration, and enabled telemetry signals. |
| Service missing from log or trace table | No data in the selected timeframe or service name attribute is missing. | Clarity timeframe, service attributes, collector received metrics, and destination status. |
| Connection issue | Integration credentials, endpoint, or namespace mismatch. | Argo CD token, Datadog key, namespace, collector pods, and Data Fidelity Validation results. |
| Signal was omitted during setup | The connection or collector was created without logs, metrics, or traces enabled. | Update the connection signal set, redeploy through the approved path, then validate received and sent telemetry. |

## Related Pages

- [Setup and Operations](setup.md)
- [Debug Connection Failures](how-to/connection-failures.md)
- [Telemetry Insights](telemetry.md)
