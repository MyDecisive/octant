# Introduction

Octant is a telemetry control plane that helps teams onboard, operate, and optimize observability pipelines without hand-assembling every component. Powered by [MyDecisive SmartHub](https://github.com/MyDecisive/mdai-hub), Octant creates the required SmartHub and OpenTelemetry collectors for you, then gives operators guided workflows for the capabilities that usually require deep knowledge of Kubernetes, OTel collectors, deployment tooling such as Argo CD, and vendor-specific telemetry setup such as Datadog.

With Octant, teams can:

1. Stand up the telemetry control plane and required collectors without manually wiring each component together.
2. Define where telemetry is collected, which signals are enabled, and where that data should go.
3. Generate deployment artifacts and manage deployment integrations from the same console used to configure the pipeline.
4. Validate that data is flowing correctly before relying on the pipeline in production.
5. Understand telemetry volume, cost, filtering, and service-level usage from the same place where controls are configured.
6. Tune runtime behavior such as enabled signals, credentials, and sampling filters without rebuilding the full deployment.

Octant also surfaces the Kubernetes resources, connection configuration, validation results, deployment artifacts, and usage data behind each capability so operators can understand what is happening and adjust it safely.

## Documentation

Use these docs to get started:

1. [Install the Octant sandbox](installation.md) to test-drive Octant locally.
2. [Understand Octant workflows](usage.md) for the main product areas.
3. [Manage connections and integrations](connections.md) for telemetry setup.
4. [Review Clarity telemetry insights and filtering](telemetry.md) for usage, cost, and sampling controls.
5. [Run Octant locally](development.md) when developing the API or embedded webapp.

To learn more about MyDecisive, [see our docs](https://docs.mydecisive.ai/).
