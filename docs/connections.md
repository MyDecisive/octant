# Connections and Integrations

Use this guide to configure integrations, create SmartHub connections, set destinations, reroute ingestion, validate telemetry, and troubleshoot missing data.

## Integrations

An integration stores reusable external system configuration. Octant currently uses Argo CD integration details for deployment operations and Datadog integration details for telemetry delivery.

## Argo CD Integration

Argo CD integrations include an integration name, Argo CD API endpoint, and Argo CD account token. In the development workflow, `octant-argo-example` can generate local setup values with:

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

After a connection exists, Octant can list connections, fetch a connection by name, delete a connection and its resources, or generate deployment manifests in JSON or YAML.

If a telemetry signal was not selected when the connection or collector was created, that signal will not appear in downstream Clarity views until the connection and workload routing are updated. Treat this as a configuration change: review the desired signal set, apply the change through the appropriate deployment path, and validate that the collector receives and sends the newly enabled signal.

## octant-demo-load for Mock Telemetry When Service Data Is Unavailable

When service data is unavailable in a development environment, use `octant-demo-load` as a demo and load-generation helper. It can emit realistic Datadog or OTLP traces into a supplied ingest endpoint.

Example development pattern:

```shell
export OCTANT_DEMO_API_KEY=<your-dd-key>
octant-demo --context=<development-cluster-context>
```

The demo tool is not required for production use.

## Reroute Ingestion Workflow

Use reroute ingestion when telemetry should flow through the SmartHub collectors managed by Octant instead of going directly from workloads to a vendor endpoint.

1. Confirm the target connection and namespace.
2. Confirm enabled telemetry signals.
3. Update workload, agent, or collector configuration to send telemetry to the SmartHub ingress endpoint.
4. Confirm the destination integration is configured.
5. Validate received and sent telemetry metrics.

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
