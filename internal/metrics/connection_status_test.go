package metrics

import (
	"testing"

	v1mock "github.com/mydecisive/mdai-gateway/internal/mock/v1"
	"github.com/mydecisive/mdai-gateway/internal/telemetry"
	"github.com/mydecisive/mdai-gateway/internal/test"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

var (
	failingFidelityResults = model.Matrix{
		{
			// failures ARE increasing
			Metric: model.Metric{
				fidelityMetricResult: fidelityCheckFail,
				fidelityMetricSignal: model.LabelValue(telemetry.Traces),
			},
			Values: []model.SamplePair{
				{
					Value: model.SampleValue(1.0),
				},
				{
					Value: model.SampleValue(2.0),
				},
			},
		},
		{
			Metric: model.Metric{
				fidelityMetricResult: fidelityCheckPass,
				fidelityMetricSignal: model.LabelValue(telemetry.Traces),
			},
			Values: []model.SamplePair{
				{
					Value: model.SampleValue(1.0),
				},
				{
					Value: model.SampleValue(2.0),
				},
			},
		},
	}

	passingFidelityResults = model.Matrix{
		{
			// failures are NOT increasing
			Metric: model.Metric{
				fidelityMetricResult: fidelityCheckFail,
				fidelityMetricSignal: model.LabelValue(telemetry.Traces),
			},
			Values: []model.SamplePair{
				{
					Value: model.SampleValue(1.0),
				},
				{
					Value: model.SampleValue(1.0),
				},
			},
		},
		{
			// passes ARE increasing
			Metric: model.Metric{
				fidelityMetricResult: fidelityCheckPass,
				fidelityMetricSignal: model.LabelValue(telemetry.Traces),
			},
			Values: []model.SamplePair{
				{
					Value: model.SampleValue(1.0),
				},
				{
					Value: model.SampleValue(2.0),
				},
			},
		},
	}
)

func TestAreSeriesValuesIncreasing(t *testing.T) {
	t.Parallel()

	t.Run("all decreasing", func(t *testing.T) {
		t.Parallel()

		areIncreasing := areSeriesValuesIncreasing(&model.SampleStream{
			Values: []model.SamplePair{
				{
					Value: model.SampleValue(1.23),
				},
				{
					Value: model.SampleValue(1.22),
				},
				{
					Value: model.SampleValue(1.21),
				},
			},
		})
		assert.False(t, areIncreasing)
	})

	t.Run("has increasing", func(t *testing.T) {
		t.Parallel()

		areIncreasing := areSeriesValuesIncreasing(&model.SampleStream{
			Values: []model.SamplePair{
				{
					Value: model.SampleValue(1.23),
				},
				{
					Value: model.SampleValue(1.22),
				},
				{
					Value: model.SampleValue(1.23),
				},
				{
					Value: model.SampleValue(0.23),
				},
			},
		})
		assert.True(t, areIncreasing)
	})
}

func TestAreMatrixValuesIncreasing(t *testing.T) {
	t.Parallel()

	type checkResults func(t *testing.T, actual bool, err error)

	testCases := []struct {
		description  string
		inputResults model.Value
		check        checkResults
	}{
		{
			description:  "nil results",
			inputResults: nil,
			check: func(t *testing.T, actual bool, err error) { // nolint: thelper
				require.False(t, actual)
				require.NoError(t, err)
			},
		},
		{
			description: "invalid result type",
			inputResults: model.Vector{
				{
					Value: model.SampleValue(1.23),
				},
				{
					Value: model.SampleValue(23.45),
				},
			},
			check: func(t *testing.T, actual bool, err error) { // nolint: thelper
				require.False(t, actual)
				require.ErrorContains(t, err, "failed to convert input result to expected type")
			},
		},
		{
			description: "not increasing",
			inputResults: model.Matrix{
				{
					Values: []model.SamplePair{
						{
							Value: model.SampleValue(1.23),
						},
						{
							Value: model.SampleValue(0.23),
						},
					},
				},
				{
					Values: []model.SamplePair{
						{
							Value: model.SampleValue(1.23),
						},
						{
							Value: model.SampleValue(1.22),
						},
					},
				},
			},
			check: func(t *testing.T, actual bool, err error) { // nolint: thelper
				require.False(t, actual)
				require.NoError(t, err)
			},
		},
		{
			description: "eventually increasing",
			inputResults: model.Matrix{
				{
					Values: []model.SamplePair{
						{
							Value: model.SampleValue(1.23),
						},
						{
							Value: model.SampleValue(0.23),
						},
					},
				},
				{
					Values: []model.SamplePair{
						{
							Value: model.SampleValue(1.23),
						},
						{
							Value: model.SampleValue(1.24),
						},
						{
							Value: model.SampleValue(1.21),
						},
					},
				},
			},
			check: func(t *testing.T, actual bool, err error) { // nolint: thelper
				require.True(t, actual)
				require.NoError(t, err)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()

			actual, err := areMatrixValuesIncreasing(testCase.inputResults)
			testCase.check(t, actual, err)
		})
	}
}

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
				Value: model.SampleValue(1.23), // > 0
			},
		}
		tracesResults := model.Vector{
			{
				Value: model.SampleValue(1.23), // > 0
			},
		}
		metricsResults := model.Vector{
			{
				Value: model.SampleValue(1.23), // > 0
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

func TestDataFidelityCheck(t *testing.T) {
	t.Parallel()

	t.Run("error - multiple failed metrics", func(t *testing.T) {
		t.Parallel()

		fidelityResults := model.Matrix{
			{
				// failures are NOT increasing
				Metric: model.Metric{
					fidelityMetricResult: fidelityCheckFail,
					fidelityMetricSignal: model.LabelValue(telemetry.Traces),
					"someLabel":          "abc123",
				},
				Values: []model.SamplePair{
					{
						Value: model.SampleValue(1.0),
					},
					{
						Value: model.SampleValue(1.0),
					},
				},
			},
			{
				// passes ARE increasing
				Metric: model.Metric{
					fidelityMetricResult: fidelityCheckFail,
					fidelityMetricSignal: model.LabelValue(telemetry.Traces),
					"someLabel":          "xyz999",
				},
				Values: []model.SamplePair{
					{
						Value: model.SampleValue(1.0),
					},
					{
						Value: model.SampleValue(2.0),
					},
				},
			},
		}

		testLogger, logOutput := test.NewTestLogger(t, zapcore.InfoLevel)
		passedCheck := dataFidelityCheck(testLogger, fidelityResults, telemetry.Traces)
		require.False(t, passedCheck)

		// validate log output
		test.ValidateOutput(t, logOutput, []test.LogOutput{
			{
				Level:   zapcore.WarnLevel,
				Message: "unable to perform data fidelity check, expected 1 set of failed and passed fidelity metric values",
			},
		})
	})

	t.Run("happy path - data fidelity is NOT good", func(t *testing.T) {
		t.Parallel()

		testLogger, _ := test.NewTestLogger(t, zapcore.InfoLevel)
		passedCheck := dataFidelityCheck(testLogger, failingFidelityResults, telemetry.Traces)
		require.False(t, passedCheck)
	})

	t.Run("happy path - data fidelity is good", func(t *testing.T) {
		t.Parallel()

		testLogger, _ := test.NewTestLogger(t, zapcore.InfoLevel)
		passedCheck := dataFidelityCheck(testLogger, passingFidelityResults, telemetry.Traces)
		require.True(t, passedCheck)
	})
}

func TestVerifyDataFidelity(t *testing.T) {
	t.Parallel()

	t.Run("error - unexpected query error", func(t *testing.T) {
		t.Parallel()

		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			QueryRange(mock.Anything, "mdai_fidelity_required_signal_checks_total", mock.Anything).
			Return(nil, nil, assert.AnError).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}
		result, err := theThing.VerifyDataFidelity(t.Context(), []telemetry.MLT{telemetry.Logs})
		require.ErrorContains(t, err, "failed to query prometheus")
		require.False(t, result)
	})

	t.Run("nil query results", func(t *testing.T) {
		t.Parallel()

		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			QueryRange(mock.Anything, "mdai_fidelity_required_signal_checks_total", mock.Anything).
			Return(nil, nil, nil).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}
		result, err := theThing.VerifyDataFidelity(t.Context(), []telemetry.MLT{telemetry.Logs})
		require.NoError(t, err)
		require.False(t, result)
	})

	t.Run("error - invalid prometheus query result", func(t *testing.T) {
		t.Parallel()

		invalidResults := model.Vector{}

		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			QueryRange(mock.Anything, "mdai_fidelity_required_signal_checks_total", mock.Anything).
			Return(invalidResults, nil, nil).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}
		result, err := theThing.VerifyDataFidelity(t.Context(), []telemetry.MLT{telemetry.Logs})
		require.ErrorContains(t, err, "failed to convert result to model.Matrix")
		require.False(t, result)
	})

	t.Run("happy path - data fidelity is NOT good", func(t *testing.T) {
		t.Parallel()
		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			QueryRange(mock.Anything, "mdai_fidelity_required_signal_checks_total", mock.Anything).
			Return(failingFidelityResults, nil, nil).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}
		result, err := theThing.VerifyDataFidelity(t.Context(), []telemetry.MLT{telemetry.Traces})
		require.NoError(t, err)
		require.False(t, result)
	})

	t.Run("happy path - data fidelity is good", func(t *testing.T) {
		t.Parallel()

		mockPromAPI := v1mock.NewMockAPI(t)
		mockPromAPI.EXPECT().
			QueryRange(mock.Anything, "mdai_fidelity_required_signal_checks_total", mock.Anything).
			Return(passingFidelityResults, nil, nil).
			Times(1)

		theThing := &ConnectionStatus{
			logger:     zaptest.NewLogger(t),
			promClient: mockPromAPI,
		}
		result, err := theThing.VerifyDataFidelity(t.Context(), []telemetry.MLT{telemetry.Traces})
		require.NoError(t, err)
		require.True(t, result)
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
		actual := buildQuery("my-conn", Ingress, telemetry.Logs)
		assert.Equal(t, expected, actual)
	})

	t.Run("Traces Egress", func(t *testing.T) {
		t.Parallel()
		expected := `increase(otelcol_exporter_sent_spans_total{exporter="datadog", mdai_connection="my-conn", service_name="my-conn-collector"}[10m])`
		actual := buildQuery("my-conn", Egress, telemetry.Traces)
		assert.Equal(t, expected, actual)
	})
}
