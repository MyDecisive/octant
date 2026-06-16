# Development

This guide covers local development for the Octant API and embedded web application.

## Kubernetes Context

Octant manages Kubernetes resources. When running outside a cluster, it expects a valid Kubernetes context in `~/.kube/config`.

```shell
kubectl config current-context
```

When running inside a cluster, Octant uses the service account, Role, and RoleBinding installed with it.

## API-Only Build

Build and run the API-only binary:

```shell
go build -trimpath -ldflags="-w -s" -o octant ./cmd/octant
./octant
```

By default, Octant listens on port `5678`. Override it with `OCTANT_PORT`.

## Embedded Webapp Build

Build the UI from the [`octant-ui`](https://github.com/MyDecisive/octant-ui) repository and copy its `dist` output into this repository:

```shell
# From the octant-ui repository.
npm install
npm run build
cp -R dist /path/to/octant/web/
```

Then build Octant with the `webapp` tag:

```shell
# From this repository.
go build -trimpath -tags webapp -ldflags="-w -s" -o octant ./cmd/octant
./octant
```

The Makefile also provides a webapp build target:

```shell
make build
```

In a webapp build, the console UI is served from `/`, RPC calls are mounted under `/api`, generated API docs are served from `/docs`, and the OpenAPI document is served from `/swagger.yaml`. When Octant is running locally, open `http://localhost:5678/docs` to view the Swagger API documentation in your browser.

## Tests

Run the Go test suite:

```shell
make test
```

Run verbose tests:

```shell
make testv
```

Run coverage:

```shell
make cover
```

## Configuration

Octant reads configuration from environment variables by default. If `OCTANT_CONFIG_PATH` is set, Octant reads that YAML file first and then applies environment overrides.

Common settings include:

| Setting | Environment variable | Default |
| --- | --- | --- |
| Server port | `OCTANT_PORT` | `5678` |
| Runtime environment | `OCTANT_ENV` | `dev` |
| Service account | `OCTANT_SERVICE_ACCOUNT` | `octant` |
| HTTP client timeout | `OCTANT_DEFAULT_TIMEOUT` | `5` seconds |
| SmartHub install timeout | `MDAI_INSTALL_TIMEOUT` | `90` seconds |
| SmartHub install poll interval | `MDAI_INSTALL_POLLING_INTERVAL_MILLIS` | `3000` milliseconds |
| Validator version | `MDAI_VALIDATOR_VERSION` | `0.1.3` |
