[![Chores](https://github.com/mydecisive/octant/actions/workflows/chores.yml/badge.svg)](https://github.com/mydecisive/octant/actions/workflows/chores.yml)
[![codecov](https://codecov.io/gh/MyDecisive/octant/graph/badge.svg?token=UPHRBSXOON)](https://codecov.io/gh/MyDecisive/octant)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/octant)](https://artifacthub.io/packages/search?repo=octant)

<section style="display: flex;">
  <div width="50%">
    <img alt="MyDecisive Logo" src="https://cdn.mydecisive.ai/media/2026/05/22/Octopus.png" width="25%"/>
  </div>
  <div width="50%">
    XXX: VIDEO HERE
  </div>
</section>


# Welcome to Octant

## ***AI DevOps that optimizes your system before incidents occur.***

Octant is the management interface for our telemetry control plane that helps teams onboard, operate, and optimize observability pipelines without hand-assembling every component. Powered by [MyDecisive SmartHub](https://github.com/MyDecisive/mdai-hub), Octant creates the required SmartHub and OpenTelemetry collectors for you, then gives operators guided workflows for the capabilities that usually require deep knowledge of Kubernetes, OTel collectors, deployment tooling such as Argo CD, and vendor-specific telemetry setup such as Datadog.

With Octant, teams can:

1. Stand up the telemetry control plane and required collectors without manually wiring each component together.
2. Define where telemetry is collected, which signals are enabled, and where that data should go.
3. Generate deployment artifacts and manage deployment integrations from the same console used to configure the pipeline.
4. Validate that data is flowing correctly before relying on the pipeline in production.
5. Understand telemetry volume, cost, filtering, and service-level usage from the same place where controls are configured.
6. Tune runtime behavior such as enabled signals, credentials, and sampling filters without rebuilding the full deployment.

Octant also surfaces the Kubernetes resources, connection configuration, validation results, deployment artifacts, and usage data behind each capability so operators can understand what is happening and adjust it safely.



Use these docs to get started:

1. [Install the Octant sandbox](docs/installation.md) to test-drive Octant locally.
2. [Understand Octant workflows](docs/usage.md) for the main product areas.
3. [Manage connections and integrations](docs/connections.md) for telemetry setup.
4. [Review Clarity telemetry insights and filtering](docs/telemetry.md) for usage, cost, and sampling controls.
5. [Run Octant locally](docs/development.md) when developing the API or embedded webapp.

To learn more about MyDecisive, [see our docs](https://docs.mydecisive.ai/).

When running the embedded webapp locally, open `http://localhost:5678/docs` to view the Swagger API documentation.
