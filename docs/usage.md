# Usage

Octant gives operators one place to install SmartHub, define telemetry connections, manage deployment and destination integrations, inspect connection health, and understand the cost and volume effects of telemetry decisions.

## Core Concepts

**SmartHub installation** is the SmartHub deployment managed by Octant in a Kubernetes namespace. Octant can start an install and stream status updates while resources are created, become ready, fail, or time out.

**Connection** is the primary SmartHub configuration object. A connection identifies the namespace and connection name, the telemetry signals to handle, the deployment mechanism, and the destinations that should receive telemetry.

**Telemetry signals** are metrics, logs, and traces. A connection can enable one or more of these signals, and Octant can update the enabled set later.

**Integration** is reusable connection information for external systems. Octant currently manages integrations for deployment and vendor integrations for telemetry delivery.

**Data Fidelity Validation** is a connection health check that records whether telemetry is flowing and whether ingress and egress payloads satisfy parity or policy expectations.

**Clarity** is Octant's telemetry usage and cost surface. It summarizes log and trace volume, cost, filtering, and per-service or per-span details across supported timeframes.

## Documentation

Use these topic guides for detailed workflows:

1. [Introduction](intro.md): product overview and documentation map.
2. [Installation](installation.md): sandbox prerequisites, setup, teardown, and deployment notes.
3. [Connections and integrations](connections.md): Argo CD, Datadog, SmartHub connections, validation, and runtime updates.
4. [Telemetry insights and filtering](telemetry.md): Clarity views, usage timeframes, sampling filters, and telemetry data sources.
5. [Development](development.md): local builds, embedded webapp builds, tests, and configuration.

## API Surface

Octant runs as a Connect RPC service. When built with the embedded web application, it also serves the console UI, the API under `/api`, and the generated API documentation under `/docs`.

Octant exposes these Connect services:

| Service | Purpose |
| --- | --- |
| `InstallService` | Install SmartHub and stream install status. |
| `ArgoCDService` | Test, save, list, and fetch Argo CD integrations. |
| `DatadogService` | Save, list, and fetch Datadog integrations. |
| `ConnectionService` | Create, list, inspect, validate, delete, and generate manifests for connections. |
| `SettingService` | Update connection runtime settings and stream progress. |
| `BudgetService` | Back Clarity queries for overall, log, and trace telemetry cost data. |
| `FilterService` | Read and update log or trace sampling filters. |
| `TimeframeService` | Query data availability for supported Clarity timeframes. |

The generated OpenAPI description is stored at `web/swagger.yaml`. In a webapp build, the generated API docs are available at `/docs`, the Swagger document is available at `/swagger.yaml`, and RPC calls are mounted under `/api`. When Octant is running locally with the embedded webapp, open `http://localhost:5678/docs` to view the Swagger API documentation in your browser.
