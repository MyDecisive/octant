package metrics

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mydecisive/mdai-gateway/internal/telemetry"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type IngressEgress int

const (
	Ingress IngressEgress = iota
	Egress
)

type collectorMetric string

const (
	logsAcceptedMetric    collectorMetric = "otelcol_receiver_accepted_log_records_total"
	logsSentMetric        collectorMetric = "otelcol_exporter_sent_log_records_total"
	metricsAcceptedMetric collectorMetric = "otelcol_receiver_accepted_metric_points_total"
	metricsSentMetric     collectorMetric = "otelcol_exporter_sent_metric_points_total"
	spansAcceptedMetric   collectorMetric = "otelcol_receiver_accepted_spans_total"
	spansSentMetric       collectorMetric = "otelcol_exporter_sent_spans_total"
)

func (ie IngressEgress) getCollectorMLTMetric(telemetryType telemetry.MLT) collectorMetric {
	// this is fugly, but gochecknoglobals decreed that my map was no good 🙃
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

func (ie IngressEgress) getComponentType() string {
	if ie == Ingress {
		return "receiver"
	}
	return "exporter"
}

const (
	fidelityCheckFail = "fail"
	fidelityCheckPass = "pass"

	fidelityMetricResult = "result"
	fidelityMetricSignal = "signal"

	tenMinutes = 10 * time.Minute //nolint: revive
)

type ConnectionStatus struct {
	promClient promv1.API
	logger     *zap.Logger
}

func NewConnectionStatus(promClient promv1.API, logger *zap.Logger) *ConnectionStatus {
	return &ConnectionStatus{
		promClient: promClient,
		logger:     logger,
	}
}

func (cs *ConnectionStatus) VerifyDataFidelity(ctx context.Context, telemetryTypes []telemetry.MLT) (bool, error) {
	// compare now to 10 minutes ago
	// cutting down on # of queries by getting all labels and filtering here.
	results, _, err := cs.promClient.QueryRange(ctx, "mdai_fidelity_required_signal_checks_total", promv1.Range{
		Start: time.Now().Add(-tenMinutes),
		End:   time.Now(),
		Step:  tenMinutes,
	})
	if err != nil {
		return false, fmt.Errorf("failed to query prometheus: %w", err)
	}
	if results == nil {
		return false, nil
	}

	resultMatrix, ok := results.(model.Matrix)
	if !ok {
		return false, errors.New("failed to convert result to model.Matrix")
	}

	for _, telemetryType := range telemetryTypes {
		if !dataFidelityCheck(cs.logger, resultMatrix, telemetryType) {
			return false, nil
		}
	}
	return true, nil
}

func dataFidelityCheck(logger *zap.Logger, resultMatrix model.Matrix, telemetryType telemetry.MLT) bool {
	failed := lo.Filter(resultMatrix, func(item *model.SampleStream, _ int) bool {
		return item.Metric[fidelityMetricResult] == fidelityCheckFail &&
			string(item.Metric[fidelityMetricSignal]) == string(telemetryType)
	})
	passed := lo.Filter(resultMatrix, func(item *model.SampleStream, _ int) bool {
		return item.Metric[fidelityMetricResult] == fidelityCheckPass &&
			string(item.Metric[fidelityMetricSignal]) == string(telemetryType)
	})

	// sanity check... this shouldn't happen.
	if len(failed) != 1 || len(passed) != 1 {
		logger.Warn("unable to perform data fidelity check, expected 1 set of failed and passed fidelity metric values")
		return false
	}

	// if the fidelity check failures are increasing OR the passed fidelity checks are NOT increasing, fail fast
	if areSeriesValuesIncreasing(failed[0]) || !areSeriesValuesIncreasing(passed[0]) {
		return false
	}
	return true
}

func buildQuery(connectionName string, ingressEgress IngressEgress, telemetryType telemetry.MLT) string {
	return fmt.Sprintf(
		"increase(%s{%s=%q, mdai_connection=%q, service_name=%q}[10m])",
		ingressEgress.getCollectorMLTMetric(telemetryType),
		ingressEgress.getComponentType(),
		"datadog", connectionName,
		connectionName+"-collector",
	)
}

func (cs *ConnectionStatus) IsTelemetryFlowing(ctx context.Context, connectionName string, ie IngressEgress, telemetryTypes []telemetry.MLT) (bool, error) {
	for _, connectionType := range telemetryTypes {
		var promQuery string
		switch connectionType {
		case telemetry.Logs:
			promQuery = buildQuery(connectionName, ie, telemetry.Logs)
		case telemetry.Traces:
			promQuery = buildQuery(connectionName, ie, telemetry.Traces)
		case telemetry.Metrics:
			promQuery = buildQuery(connectionName, ie, telemetry.Metrics)
		default:
			return false, fmt.Errorf("unknown telemetry type: %s", connectionType)
		}

		// compare the last minute of results
		results, _, err := cs.promClient.Query(ctx, promQuery, time.Now())
		if err != nil {
			return false, fmt.Errorf("failed to query prometheus: %w", err)
		}

		resultVector, ok := results.(model.Vector)
		if !ok || len(resultVector) == 0 {
			return false, nil
		}
		metricsIncreasing := float64(resultVector[0].Value) > 0

		// return immediately if one of the telemetry types isn't increasing, we don't need to keep checking
		if !metricsIncreasing {
			return false, nil
		}
	}
	return true, nil
}

func areMatrixValuesIncreasing(results model.Value) (bool, error) {
	if results == nil {
		return false, nil
	}
	resultMatrix, ok := results.(model.Matrix)
	if !ok {
		return false, errors.New("failed to convert input result to expected type")
	}

	if slices.ContainsFunc(resultMatrix, areSeriesValuesIncreasing) {
		return true, nil
	}
	return false, nil
}

func areSeriesValuesIncreasing(series *model.SampleStream) bool {
	for i := 1; i < len(series.Values); i++ {
		prev := series.Values[i-1]
		curr := series.Values[i]

		diff := float64(curr.Value) - float64(prev.Value)

		if diff > 0 {
			// we can return immediately if the values went up in our time range, no need to keep going.
			return true
		}
	}
	return false
}
