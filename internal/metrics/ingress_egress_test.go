package metrics

import (
	"testing"

	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestGetCollectorMetric(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		telemetryType telemetry.MLT
		ingressEgress IngressEgress
		expected      collectorMetric
	}{
		{"Logs Ingress", telemetry.Logs, Ingress, logsAcceptedMetric},
		{"Logs Egress", telemetry.Logs, Egress, logsSentMetric},
		{"Metrics Ingress", telemetry.Metrics, Ingress, metricsAcceptedMetric},
		{"Metrics Egress", telemetry.Metrics, Egress, metricsSentMetric},
		{"Traces Ingress", telemetry.Traces, Ingress, spansAcceptedMetric},
		{"Traces Egress", telemetry.Traces, Egress, spansSentMetric},
		{"Unknown Type", "unknown", Ingress, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual := tc.ingressEgress.getCollectorMLTMetric(tc.telemetryType)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestGetReceiverExporter(t *testing.T) {
	t.Parallel()

	t.Run("Ingress returns receiver", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "receiver", Ingress.getComponentType())
	})

	t.Run("Egress returns exporter", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "exporter", Egress.getComponentType())
	})
}

func TestBuildQuery(t *testing.T) {
	t.Parallel()

	t.Run("Logs Ingress", func(t *testing.T) {
		t.Parallel()
		expected := `increase(otelcol_receiver_accepted_log_records_total{` +
			`receiver="datadog", mdai_connection="my-conn", service_name="my-conn-sampling-lb-collector"}[10m])`
		actual := buildFlowQuery("my-conn", Ingress, telemetry.Logs)
		assert.Equal(t, expected, actual)
	})

	t.Run("Trace Ingress", func(t *testing.T) {
		t.Parallel()
		expected := `increase(otelcol_receiver_accepted_spans_total{` +
			`receiver="datadog", mdai_connection="my-conn", service_name="my-conn-sampling-lb-collector"}[10m])`
		actual := buildFlowQuery("my-conn", Ingress, telemetry.Traces)
		assert.Equal(t, expected, actual)
	})

	t.Run("Logs Egress", func(t *testing.T) {
		t.Parallel()
		expected := `increase(otelcol_exporter_sent_log_records_total{` +
			`exporter="datadog", mdai_connection="my-conn", service_name="my-conn-log-sampling-collector"}[10m])`
		actual := buildFlowQuery("my-conn", Egress, telemetry.Logs)
		assert.Equal(t, expected, actual)
	})

	t.Run("Traces Egress", func(t *testing.T) {
		t.Parallel()
		expected := `increase(otelcol_exporter_sent_spans_total{` +
			`exporter="datadog", mdai_connection="my-conn", service_name="my-conn-trace-sampling-collector"}[10m])`
		actual := buildFlowQuery("my-conn", Egress, telemetry.Traces)
		assert.Equal(t, expected, actual)
	})
}
