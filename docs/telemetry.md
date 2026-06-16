# Telemetry Insights (Clarity)

Octant's Clarity applet helps you understand telemetry volume, cost, and filtering decisions for logs and traces so teams can tune telemetry controls from the same place they inspect usage.

## Clarity Usage

Clarity summarizes telemetry usage for supported timeframes:

1. Last 24 hours.
2. Month to date.
3. Last month.

The overall Clarity view reports combined log and trace cost, received volume, sent volume, filtered amount, cost rate, and percentage of total cost.

The log view reports service-level log data, including service name, sent data in GB, percentage of total log cost, and cost.

The trace view reports root span usage, including span name, breadth, depth, invocation count, and cost.

Octant also reports timeframe status so the UI can distinguish ready data from missing or insufficient data.

## Sampling Filters

Octant manages log and trace filters per connection. Each filter includes:

1. Filter type: log or trace.
2. Sampled percentage.
3. Whether errors should always be included.

Filter updates are streamed. Octant reports progress while the value is updated, while propagation is pending, and when the update is complete.

Default sampling settings can be configured with:

| Setting | Environment variable | Default |
| --- | --- | --- |
| Default log sampling ratio | `OCTANT_DEFAULT_LOG_SAMPLING_RATIO` | `100` |
| Default log include errors | `OCTANT_DEFAULT_LOG_INCLUDE_ERR` | `true` |
| Default trace sampling ratio | `OCTANT_DEFAULT_TRACE_SAMPLING_RATIO` | `100` |
| Default trace include errors | `OCTANT_DEFAULT_TRACE_INCLUDE_ERR` | `true` |

## Data Sources

Octant uses Prometheus and GreptimeDB access for connection status, validation, and cost data.

Common data source overrides include:

| Setting | Environment variable | Default |
| --- | --- | --- |
| Prometheus URL override | `PROMETHEUS_URL` | unset |
| Prometheus service name | `PROMETHEUS_SERVICE_NAME` | `prometheus-operated` |
| Prometheus port | `PROMETHEUS_PORT` | `9090` |
| GreptimeDB URL override | `OCTANT_GREPTIMEDB_URL` | unset |
| Default GreptimeDB service | `OCTANT_DEFAULT_GREPTIMEDB` | `mdai-greptimedb` |
