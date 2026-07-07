# API Reference

The Octant Connect RPC API supports installation, integrations, connections, settings, validation, and Clarity workflows.

## Source of Truth

- Swagger/OpenAPI source: `octant/web/swagger.yaml`.
- The embedded webapp serves generated API documentation under `/docs` and the OpenAPI document under `/swagger.yaml`.
- RPC calls are mounted under `/api` in the webapp build.

## Usage Notes

- Octant exposes Connect RPC services with generated OpenAPI documentation.
- Most unary operations appear as both `GET` and `POST` in the Swagger document.
- Streaming or update operations may only expose `POST` with Connect-compatible content types.
- Requests and responses are defined by schemas under `components.schemas` in `web/swagger.yaml`.

## Service Summary

| Service | Purpose | Operations |
| --- | --- | --- |
| `InstallService` | Install SmartHub and inspect installation status. | `GetInstallStatus`, `InstallMDAIHub` |
| `ArgoCDService` | Manage and test Argo CD deployment integrations. | `GetArgoIntegrationByName`, `GetArgoIntegrations`, `SaveArgoConnection`, `TestConnection` |
| `DatadogService` | Manage Datadog destination integrations. | `GetDatadogIntegrationByName`, `GetDatadogIntegrations`, `SaveDatadogIntegration` |
| `ConnectionService` | Create, inspect, validate, delete, and generate manifests for connections. | `CreateConnection`, `CreateConnectionValidatorRun`, `DeleteConnection`, `DeleteConnectionValidator`, `GenerateManifests`, `GetConnection`, `GetConnectionStatus`, `GetConnectionValidatorRunIds`, `GetConnections` |
| `SettingService` | Update connection runtime settings. | `Update` |
| `BudgetService` | Query Clarity budget, volume, and cost data. | `Log`, `Overall`, `Trace` |
| `FilterService` | Read and update log or trace sampling filters. | `GetFilter`, `UpdateFilter` |
| `TimeframeService` | Query Clarity timeframe availability. | `TimeframeStatus` |

## InstallService

Install SmartHub and inspect installation status.

| Operation | HTTP | Path | Request | Response | Description |
| --- | --- | --- | --- | --- | --- |
| `GetInstallStatus` | POST | `/octant.v1alpha.InstallService/GetInstallStatus` | `octant.v1alpha.GetInstallStatusRequest` | `octant.v1alpha.GetInstallStatusResponse` | GetInstallStatus creates a response stream with mdai install status updates. |
| `InstallMDAIHub` | POST | `/octant.v1alpha.InstallService/InstallMDAIHub` | `octant.v1alpha.InstallMDAIHubRequest` | `google.protobuf.Empty` | InstallMDAIHub initiates installing the mdai smart hub with the provided request data. |

## ArgoCDService

Manage and test Argo CD deployment integrations.

| Operation | HTTP | Path | Request | Response | Description |
| --- | --- | --- | --- | --- | --- |
| `GetArgoIntegrationByName` | GET, POST | `/octant.v1alpha.ArgoCDService/GetArgoIntegrationByName` | `octant.v1alpha.GetArgoIntegrationByNameRequest` | `octant.v1alpha.GetArgoIntegrationByNameResponse` | GetIntegrations returns list of argo integration names. |
| `GetArgoIntegrations` | GET, POST | `/octant.v1alpha.ArgoCDService/GetArgoIntegrations` | `google.protobuf.Empty` | `octant.v1alpha.GetArgoIntegrationsResponse` | GetIntegrations returns list of argo integration names. |
| `SaveArgoConnection` | POST | `/octant.v1alpha.ArgoCDService/SaveArgoConnection` | `octant.v1alpha.SaveArgoConnectionRequest` | `google.protobuf.Empty` | SaveArgoConnection saves the argo connection details. |
| `TestConnection` | POST | `/octant.v1alpha.ArgoCDService/TestConnection` | `octant.v1alpha.TestConnectionRequest` | `octant.v1alpha.TestConnectionResponse` | TestConnection uses the provided request data to test the argo connection. |

## DatadogService

Manage Datadog destination integrations.

| Operation | HTTP | Path | Request | Response | Description |
| --- | --- | --- | --- | --- | --- |
| `GetDatadogIntegrationByName` | GET, POST | `/octant.v1alpha.DatadogService/GetDatadogIntegrationByName` | `octant.v1alpha.GetDatadogIntegrationByNameRequest` | `octant.v1alpha.GetDatadogIntegrationByNameResponse` | GetDatadogIntegrationByName returns list of datadog integration names. |
| `GetDatadogIntegrations` | GET, POST | `/octant.v1alpha.DatadogService/GetDatadogIntegrations` | `google.protobuf.Empty` | `octant.v1alpha.GetDatadogIntegrationsResponse` | GetIntegrations returns list of datadog integration names. |
| `SaveDatadogIntegration` | POST | `/octant.v1alpha.DatadogService/SaveDatadogIntegration` | `octant.v1alpha.SaveDatadogIntegrationRequest` | `google.protobuf.Empty` | SaveDatadogIntegration saves the given datadog integration. If the integration already exists, this will override the saved data with the one provided. |

## ConnectionService

Create, inspect, validate, delete, and generate manifests for connections.

| Operation | HTTP | Path | Request | Response | Description |
| --- | --- | --- | --- | --- | --- |
| `CreateConnection` | POST | `/octant.v1alpha.ConnectionService/CreateConnection` | `octant.v1alpha.CreateConnectionRequest` | `google.protobuf.Empty` | CreateConnection creates a connection and deploys it |
| `CreateConnectionValidatorRun` | POST | `/octant.v1alpha.ConnectionService/CreateConnectionValidatorRun` | `octant.v1alpha.CreateConnectionValidatorRunRequest` | `octant.v1alpha.CreateConnectionValidatorRunResponse` | CreateConnectionValidatorRun creates a new validator run for the given connection. Will create a new validator run if one already exists. |
| `DeleteConnection` | POST | `/octant.v1alpha.ConnectionService/DeleteConnection` | `octant.v1alpha.DeleteConnectionRequest` | `google.protobuf.Empty` | DeleteConnection removes an existing connection and its associated resources |
| `DeleteConnectionValidator` | POST | `/octant.v1alpha.ConnectionService/DeleteConnectionValidator` | `octant.v1alpha.DeleteConnectionValidatorRequest` | `google.protobuf.Empty` | DeleteConnectionValidator deletes connection validator resources |
| `GenerateManifests` | POST | `/octant.v1alpha.ConnectionService/GenerateManifests` | `octant.v1alpha.GenerateManifestsRequest` | `octant.v1alpha.GenerateManifestsResponse` | GenerateManifests generates the manifest base on the given input and returns the compressed zip file as a byte stream. Note: To create a download link in the FE, the stream of file content data should be stored in a typed blob and then https://developer.mozilla.org/en-US/docs/Web/API/URL/createObjectURL_static should be used. |
| `GetConnection` | GET, POST | `/octant.v1alpha.ConnectionService/GetConnection` | `octant.v1alpha.GetConnectionRequest` | `octant.v1alpha.GetConnectionResponse` | GetConnection gets details about a connection |
| `GetConnectionStatus` | GET, POST | `/octant.v1alpha.ConnectionService/GetConnectionStatus` | `octant.v1alpha.GetConnectionStatusRequest` | `octant.v1alpha.GetConnectionStatusResponse` | GetConnectionStatus gets the status of a connection based on dataflow and validation metrics |
| `GetConnectionValidatorRunIds` | GET, POST | `/octant.v1alpha.ConnectionService/GetConnectionValidatorRunIds` | `octant.v1alpha.GetConnectionValidatorRunIdsRequest` | `octant.v1alpha.GetConnectionValidatorRunIdsResponse` | GetConnectionValidatorRuns gets the validator runs that exist in the validation dataset |
| `GetConnections` | GET, POST | `/octant.v1alpha.ConnectionService/GetConnections` | `google.protobuf.Empty` | `octant.v1alpha.GetConnectionsResponse` | GetConnections gets all existing connection names |

## SettingService

Update connection runtime settings.

| Operation | HTTP | Path | Request | Response | Description |
| --- | --- | --- | --- | --- | --- |
| `Update` | POST | `/octant.v1alpha.SettingService/Update` | `octant.v1alpha.UpdateRequest` | `octant.v1alpha.UpdateResponse` | Update updates the relevant configmap/secret and then redeploy necessary collectors to relfect the changes. If an update is in progress, this will immediately return with an error code `unavailable`. Otherwise, this will continuously give update until the update is complete or errored. Note: Please take a look at https://connectrpc.com/docs/protocol/#error-codes for all other possible error codes. |

## BudgetService

Query Clarity budget, volume, and cost data.

| Operation | HTTP | Path | Request | Response | Description |
| --- | --- | --- | --- | --- | --- |
| `Log` | GET, POST | `/budget.v1alpha.BudgetService/Log` | `budget.v1alpha.LogRequest` | `budget.v1alpha.LogResponse` | Log returns budget stats for logs. |
| `Overall` | GET, POST | `/budget.v1alpha.BudgetService/Overall` | `budget.v1alpha.OverallRequest` | `budget.v1alpha.OverallResponse` | Overall returns budeget stats. |
| `Trace` | GET, POST | `/budget.v1alpha.BudgetService/Trace` | `budget.v1alpha.TraceRequest` | `budget.v1alpha.TraceResponse` | Trace returns budget stats for traces. |

## FilterService

Read and update log or trace sampling filters.

| Operation | HTTP | Path | Request | Response | Description |
| --- | --- | --- | --- | --- | --- |
| `GetFilter` | GET, POST | `/budget.v1alpha.FilterService/GetFilter` | `budget.v1alpha.GetFilterRequest` | `budget.v1alpha.GetFilterResponse` | GetFilter returns the current filter setting of the given filter type. If  an update is in progress, this will return with an error code `unavailable`. |
| `UpdateFilter` | POST | `/budget.v1alpha.FilterService/UpdateFilter` | `budget.v1alpha.UpdateFilterRequest` | `budget.v1alpha.UpdateFilterResponse` | UpdateFilter attempts to update filter setting of the given filter type. If an update is in progress, this will immediately return with an error code `unavailable`. Otherwise, this will continuously give update until the update is complete or errored. |

## TimeframeService

Query Clarity timeframe availability.

| Operation | HTTP | Path | Request | Response | Description |
| --- | --- | --- | --- | --- | --- |
| `TimeframeStatus` | GET, POST | `/budget.v1alpha.TimeframeService/TimeframeStatus` | `budget.v1alpha.TimeframeStatusRequest` | `budget.v1alpha.TimeframeStatusResponse` | TimeframeStatus returns timeframe status for each timeframe. |

## Related Pages

- [Setup and Operations](setup.md)
- [Connections and Integrations](connections.md)
- [Telemetry Insights](telemetry.md)
