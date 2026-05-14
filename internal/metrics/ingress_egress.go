package metrics

import "github.com/mydecisive/octant/internal/telemetry"

type IngressEgress int

const (
	Ingress IngressEgress = iota
	Egress
)

func (ie IngressEgress) getCollectorMLTMetric(telemetryType telemetry.MLT) collectorMetric {
	switch telemetryType {
	case telemetry.Logs:
		if ie == Ingress {
			return logsAcceptedMetric
		}
		return logsSentMetric
	case telemetry.Metrics:
		if ie == Ingress {
			return metricsAcceptedMetric
		}
		return metricsSentMetric
	case telemetry.Traces:
		if ie == Ingress {
			return spansAcceptedMetric
		}
		return spansSentMetric
	default:
		return ""
	}
}

func (ie IngressEgress) getServiceName(telemetryType telemetry.MLT, connectionName string) string {
	// NOTE: this value HAS TO stay in sync with the `service.telemetry.resource.service.name` value
	// over in `internal/connection/templates/lb-collector.yaml.tmpl`
	if ie == Ingress {
		return connectionName + "-sampling-lb-collector"
	}

	switch telemetryType {
	case telemetry.Logs:
		return connectionName + "-log-sampling-collector"
	case telemetry.Traces:
		return connectionName + "-trace-sampling-collector"
	default:
		return ""
	}
}

func (ie IngressEgress) getComponentType() string {
	if ie == Ingress {
		return "receiver"
	}
	return "exporter"
}
