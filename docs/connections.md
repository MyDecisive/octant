# Connections and Integrations

Connections define how SmartHub handles telemetry for a named deployment. Integrations provide the external system credentials and endpoints that connections use for deployment and telemetry delivery.

## Integrations

Octant manages reusable integrations for external systems.

Argo CD integrations are used for deployment operations and include:

1. Integration name.
2. Argo CD API endpoint.
3. Argo CD account token.

Datadog integrations are used as telemetry destinations and include:

1. Integration name.
2. Datadog API URL.
3. Datadog API key.

Octant can list saved integrations and fetch an integration by name. For Argo CD, Octant can also test a connection before saving it.

## Connections

A connection includes:

1. Scope: namespace and connection name.
2. Telemetry types: metrics, logs, traces, or a combination.
3. Deployment: deployment type and integration name.
4. Destinations: telemetry destinations and their integration names.

Supported deployment types are:

1. Argo CD side-load.
2. Argo CD manifest-based deployment.

Supported manifest output formats are JSON and YAML.

After a connection exists, Octant can list all connections, fetch a connection by name, delete a connection and its resources, or generate downloadable deployment manifests.

## Validation

Octant can create validator runs for a connection and retrieve validator run IDs already present in the validation dataset.

Connection status reports:

1. Whether the collector is receiving telemetry.
2. Whether the collector is sending telemetry.
3. Whether clients are connected for the configured telemetry types.
4. Whether telemetry satisfies data integrity checks.
5. Per-signal validation results for logs, metrics, and traces.

Validation results include parity and policy checks, plus per-attribute parity and policy details when available.

## Runtime Settings

Octant can update runtime settings for an existing connection without recreating the whole configuration.

Supported updates include:

1. Enabled telemetry types.
2. Datadog API URL.
3. Datadog API key.

When settings change, Octant updates the relevant ConfigMap or Secret and redeploys the necessary collectors. The update stream reports progress through update, deploy, wait, completion, or timeout states.
