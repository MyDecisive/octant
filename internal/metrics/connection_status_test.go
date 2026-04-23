package metrics

import (
	"strings"
	"testing"

	v1mock "github.com/mydecisive/octant/internal/mock/v1"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestIsTelemetryFlowing(t *testing.T) {
	t.Parallel()

	t.Run("error - invalid MLT type", func(t *testing.T) {
		t.Parallel()

		mockPromAPI := v1mock.NewMockAPI(t)
		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}

		actual, err := theThing.IsTelemetryFlowing(t.Context(), "foobar", Ingress, []telemetry.MLT{"invalid", telemetry.Logs, telemetry.Traces})
		require.False(t, actual)
		require.ErrorContains(t, err, "unknown telemetry type: invalid")
	})

	t.Run("error querying prometheus API", func(t *testing.T) {
		t.Parallel()

		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			Query(mock.Anything, `increase(otelcol_receiver_accepted_log_records_total{receiver="datadog", mdai_connection="foobar", service_name="foobar-collector"}[10m])`, mock.Anything).
			Return(nil, nil, assert.AnError).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}

		actual, err := theThing.IsTelemetryFlowing(t.Context(), "foobar", Ingress, []telemetry.MLT{telemetry.Logs, telemetry.Traces})
		require.False(t, actual)
		require.ErrorContains(t, err, "failed to query prometheus")
	})

	t.Run("empty query results", func(t *testing.T) {
		t.Parallel()

		queryResults := model.Vector{}
		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			Query(mock.Anything, `increase(otelcol_receiver_accepted_log_records_total{receiver="datadog", mdai_connection="foobar", service_name="foobar-collector"}[10m])`, mock.Anything).
			Return(queryResults, nil, nil).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}

		actual, err := theThing.IsTelemetryFlowing(t.Context(), "foobar", Ingress, []telemetry.MLT{telemetry.Logs, telemetry.Traces})
		require.False(t, actual)
		require.NoError(t, err)
	})

	t.Run("happy path - not increasing", func(t *testing.T) {
		t.Parallel()

		logsResults := model.Vector{
			{
				Value: model.SampleValue(1.23), // > 0 means increasing
			},
		}
		tracesResults := model.Vector{
			{
				Value: model.SampleValue(0.0), // <= 0 means NOT increasing
			},
		}
		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			Query(mock.Anything, `increase(otelcol_receiver_accepted_log_records_total{receiver="datadog", mdai_connection="foobar", service_name="foobar-collector"}[10m])`, mock.Anything).
			Return(logsResults, nil, nil).
			Times(1)
		mockPromAPI.EXPECT().
			Query(mock.Anything, `increase(otelcol_receiver_accepted_spans_total{receiver="datadog", mdai_connection="foobar", service_name="foobar-collector"}[10m])`, mock.Anything).
			Return(tracesResults, nil, nil).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}

		actual, err := theThing.IsTelemetryFlowing(t.Context(), "foobar", Ingress, []telemetry.MLT{telemetry.Logs, telemetry.Traces})
		require.False(t, actual)
		require.NoError(t, err)
	})

	t.Run("happy path - increasing", func(t *testing.T) {
		t.Parallel()

		logsResults := model.Vector{
			{
				Value: model.SampleValue(1.23),
			},
		}
		tracesResults := model.Vector{
			{
				Value: model.SampleValue(1.23),
			},
		}
		metricsResults := model.Vector{
			{
				Value: model.SampleValue(1.23),
			},
		}
		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			Query(mock.Anything, `increase(otelcol_receiver_accepted_log_records_total{receiver="datadog", mdai_connection="foobar", service_name="foobar-collector"}[10m])`, mock.Anything).
			Return(logsResults, nil, nil).
			Times(1)
		mockPromAPI.EXPECT().
			Query(mock.Anything, `increase(otelcol_receiver_accepted_spans_total{receiver="datadog", mdai_connection="foobar", service_name="foobar-collector"}[10m])`, mock.Anything).
			Return(tracesResults, nil, nil).
			Times(1)
		mockPromAPI.EXPECT().
			Query(mock.Anything, `increase(otelcol_receiver_accepted_metric_points_total{receiver="datadog", mdai_connection="foobar", service_name="foobar-collector"}[10m])`, mock.Anything).
			Return(metricsResults, nil, nil).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}

		actual, err := theThing.IsTelemetryFlowing(t.Context(), "foobar", Ingress, []telemetry.MLT{telemetry.Logs, telemetry.Traces, telemetry.Metrics})
		require.True(t, actual)
		require.NoError(t, err)
	})
}

func TestVerifyDataFidelity(t *testing.T) {
	t.Parallel()

	t.Run("error - unexpected while querying prometheus", func(t *testing.T) {
		t.Parallel()

		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			Query(mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil, assert.AnError).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}
		result, _, err := theThing.VerifyDataFidelity(t.Context(), "test-conn", []telemetry.MLT{telemetry.Logs})

		require.ErrorContains(t, err, "checking attribute parity fidelity")
		require.False(t, result)
	})

	t.Run("nil query results (no data) evaluates to false", func(t *testing.T) {
		t.Parallel()

		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			Query(mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil, nil).
			Times(4) // 2 for attributes, 2 for signals

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}
		result, validations, err := theThing.VerifyDataFidelity(t.Context(), "test-conn", []telemetry.MLT{telemetry.Logs})
		require.NoError(t, err)
		require.False(t, result)
		require.False(t, validations[telemetry.Logs].Parity)
		require.False(t, validations[telemetry.Logs].Policy)
	})

	t.Run("error - invalid prometheus query result", func(t *testing.T) {
		t.Parallel()

		invalidResults := model.Matrix{}

		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			Query(mock.Anything, mock.Anything, mock.Anything).
			Return(invalidResults, nil, nil).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}
		result, _, err := theThing.VerifyDataFidelity(t.Context(), "test-conn", []telemetry.MLT{telemetry.Logs})
		require.ErrorContains(t, err, "failed to convert result to model.Vector")
		require.False(t, result)
	})

	t.Run("data integrity is false when BOTH signal parity and policy fail", func(t *testing.T) {
		t.Parallel()

		failVector := model.Vector{
			{
				Metric: model.Metric{
					fidelityMetricResult: fidelityCheckFail,
					fidelityMetricSignal: model.LabelValue(telemetry.Traces),
				},
				Value: 5.0,
			},
		}

		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			Query(mock.Anything, mock.Anything, mock.Anything).
			Return(failVector, nil, nil).
			Times(4)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}
		result, validations, err := theThing.VerifyDataFidelity(t.Context(), "test-conn", []telemetry.MLT{telemetry.Traces})
		require.NoError(t, err)
		require.False(t, result)
		require.False(t, validations[telemetry.Traces].Parity)
		require.False(t, validations[telemetry.Traces].Policy)
	})

	t.Run("data integrity is true when ONLY ONE signal check fails", func(t *testing.T) {
		t.Parallel()

		passVector := model.Vector{
			{
				Metric: model.Metric{
					fidelityMetricResult: fidelityCheckPass,
					fidelityMetricSignal: model.LabelValue(telemetry.Traces),
				},
				Value: 5.0,
			},
		}
		failVector := model.Vector{
			{
				Metric: model.Metric{
					fidelityMetricResult: fidelityCheckFail,
					fidelityMetricSignal: model.LabelValue(telemetry.Traces),
				},
				Value: 5.0,
			},
		}

		mockPromAPI := v1mock.NewMockAPI(t)

		mockPromAPI.EXPECT().
			Query(mock.Anything, mock.MatchedBy(func(q string) bool { return strings.Contains(q, "attribute") }), mock.Anything).
			Return(passVector, nil, nil).
			Times(2)

		mockPromAPI.EXPECT().
			Query(mock.Anything, mock.MatchedBy(func(q string) bool { return strings.Contains(q, "mdai_fidelity_signal_checks_total") }), mock.Anything).
			Return(failVector, nil, nil).
			Times(1)

		mockPromAPI.EXPECT().
			Query(mock.Anything, mock.MatchedBy(func(q string) bool { return strings.Contains(q, "mdai_fidelity_required_signal_checks_total") }), mock.Anything).
			Return(passVector, nil, nil).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}
		result, validations, err := theThing.VerifyDataFidelity(t.Context(), "test-conn", []telemetry.MLT{telemetry.Traces})

		require.NoError(t, err)
		require.True(t, result)
		require.False(t, validations[telemetry.Traces].Parity)
		require.True(t, validations[telemetry.Traces].Policy)
	})

	t.Run("data integrity is true when signals pass but attributes fail", func(t *testing.T) {
		t.Parallel()

		passVector := model.Vector{
			{
				Metric: model.Metric{
					fidelityMetricResult: fidelityCheckPass,
					fidelityMetricSignal: model.LabelValue(telemetry.Traces),
				},
				Value: 5.0,
			},
		}
		failAttrVector := model.Vector{
			{
				Metric: model.Metric{
					fidelityMetricResult:    fidelityCheckFail,
					fidelityMetricSignal:    model.LabelValue(telemetry.Traces),
					fidelityMetricAttribute: "span_id",
				},
				Value: 5.0,
			},
		}

		mockPromAPI := v1mock.NewMockAPI(t)

		mockPromAPI.EXPECT().
			Query(mock.Anything, mock.MatchedBy(func(q string) bool { return strings.Contains(q, "attribute") }), mock.Anything).
			Return(failAttrVector, nil, nil).
			Times(2)

		mockPromAPI.EXPECT().
			Query(mock.Anything, mock.MatchedBy(func(q string) bool { return strings.Contains(q, "signal") && !strings.Contains(q, "attribute") }), mock.Anything).
			Return(passVector, nil, nil).
			Times(2)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}
		result, validations, err := theThing.VerifyDataFidelity(t.Context(), "test-conn", []telemetry.MLT{telemetry.Traces})

		require.NoError(t, err)
		require.True(t, result)
		require.True(t, validations[telemetry.Traces].Parity)
		require.True(t, validations[telemetry.Traces].Policy)

		val, exists := validations[telemetry.Traces].Attributes.Parity["span_id"]
		require.True(t, exists)
		require.False(t, val)
	})
}

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
		expected := `increase(otelcol_receiver_accepted_log_records_total{receiver="datadog", mdai_connection="my-conn", service_name="my-conn-collector"}[10m])`
		actual := buildFlowQuery("my-conn", Ingress, telemetry.Logs)
		assert.Equal(t, expected, actual)
	})

	t.Run("Traces Egress", func(t *testing.T) {
		t.Parallel()
		expected := `increase(otelcol_exporter_sent_spans_total{exporter="datadog", mdai_connection="my-conn", service_name="my-conn-collector"}[10m])`
		actual := buildFlowQuery("my-conn", Egress, telemetry.Traces)
		assert.Equal(t, expected, actual)
	})
}
