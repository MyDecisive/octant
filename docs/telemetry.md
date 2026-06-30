# Telemetry Insights

Use this guide to understand Clarity, review telemetry volume and cost, tune log and trace filtering, and troubleshoot missing service data.

## Clarity Usage

Clarity is Octant's product surface for understanding telemetry usage. It helps operators answer:

- Which services or telemetry sources are driving the most volume or cost.
- Whether log or trace reduction settings are active.
- Whether error telemetry remains protected while sampling reduces routine telemetry.
- Whether enough data exists for the selected time range.

Clarity should be used after a connection is created and telemetry is flowing through SmartHub. If a collector was created without logs or traces enabled, first update the connection or collector configuration so the missing signal is routed through SmartHub, then wait for the selected time range to accumulate data.

## Data Readiness

Clarity depends on telemetry flowing through an enabled SmartHub connection and on enough historical data for the selected time range. If data is missing, confirm the connection first, then confirm that the selected time range has had enough time to accumulate data.

## Supported Time Ranges

Supported time ranges include:

- Last 24 hours.
- Month to date.
- Last month.

Octant also reports timeframe status so the UI can distinguish ready data from missing or insufficient data.

When a time range has insufficient data, Clarity results can be empty or incomplete even when telemetry is currently flowing. Check the most recent range first, then compare broader ranges after enough historical data is available.

## Top Talkers

Top Talkers are the services, logs, spans, or other telemetry sources contributing the most volume or cost in the selected Clarity view and timeframe. Use Top Talkers to decide where a filtering, sampling, or instrumentation change will have the largest impact.

## Span Breadth

Span breadth describes how wide a trace is at a span or root-span level. It helps operators understand fan-out and trace shape when reviewing trace cost and volume. High breadth can point to a request that fans out to many downstream operations.

## Span Depth

Span depth describes how deep a trace path is. It helps operators understand call-chain length and where trace complexity may affect telemetry volume. High depth can point to long request chains that may be expensive to retain in full detail.

## Invocations

Invocations are counted occurrences of a span, root span, service operation, or trace pattern in the selected timeframe.

## Reducing Logs or Traces by Volume

Reducing logs or traces by volume means lowering the amount of telemetry sent onward while preserving useful operational signal. Octant supports sampling filters that can reduce log or trace volume by sampled percentage.

Use reduction settings when volume or cost is higher than expected, but validate the result after the change. A successful reduction should lower sent volume while preserving required error telemetry and enough representative logs or traces for troubleshooting.

## Always Keep Errors

Always keep errors means error telemetry should remain included even when a sampling filter reduces normal log or trace volume. This protects high-value failure signals while lowering routine telemetry volume.

Keep this setting enabled for production-oriented workflows unless an SME confirms a different policy. Error telemetry is usually the highest-value signal during incident review.

## Why a Service May Be Missing from Log or Trace Tables

A service may be missing from log or trace tables when:

- The selected timeframe has no data for that service.
- The service is not emitting logs or traces.
- Telemetry is not routed through the SmartHub connection.
- Service-name attributes are missing or inconsistent.
- Sampling or filtering removed too much low-volume data.
- The selected timeframe does not have enough historical data yet.

Start troubleshooting by confirming the selected time range, then check whether the service emits the expected signal and whether that signal is enabled on the SmartHub connection.

## Intelligent LogStream Tuning

Intelligent LogStream tuning adjusts log sampling and filtering to reduce volume while preserving important log data such as errors.

## Intelligent Trace Sampling

Intelligent Trace Sampling adjusts trace sampling to reduce trace volume while preserving important traces such as errors or representative high-value requests.

## Sampling Filters

Each filter includes:

- Filter type: log or trace.
- Sampled percentage.
- Whether errors should always be included.

Filter updates are streamed. Octant reports progress while the value is updated, while propagation is pending, and when the update is complete.

After changing a filter, validate both the control-plane update and the data effect:

1. Confirm the update completed in Octant.
2. Confirm SmartHub collectors remain healthy.
3. Compare received and sent volume for the affected signal.
4. Confirm error telemetry still appears when `always keep errors` is enabled.

## Volume and Cost Audit

Use a volume and cost audit to compare received volume, sent volume, filtered amount, cost rate, and percentage of total cost across supported time ranges.

1. Select the timeframe.
2. Review total log and trace cost.
3. Identify Top Talkers.
4. Check sampling settings.
5. Adjust filters through change control.
6. Validate that cost and volume moved in the expected direction without losing required error telemetry.

## Estimated Cost and Data Charges

Estimated cost and data charge views are planning signals, not billing statements. Use them to identify high-volume services, compare the effect of sampling changes, and prioritize follow-up with service owners.

## Connection Issues from Clarity

If Clarity shows missing or stale data, verify the connection before changing sampling settings:

| Symptom | First check | Related page |
| --- | --- | --- |
| No services appear | Selected timeframe and enabled telemetry signals. | [Connections and Integrations](connections.md) |
| Logs appear but traces do not | Whether traces were enabled when the connection or collector was created. | [Connections and Integrations](connections.md) |
| Cost looks unchanged after filtering | Filter update status and enough data in the selected timeframe. | [Setup and Operations](setup.md) |
| Error rows are missing | Whether the service emits error attributes and whether `always keep errors` is enabled. | [Connections and Integrations](connections.md) |

## Related Pages

- [Connections and Integrations](connections.md)
- [Setup and Operations](setup.md)
- [Architecture](architecture.md)
