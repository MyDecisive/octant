package metrics

import (
	"context"
	"errors"
	"fmt"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"time"

	"github.com/mydecisive/octant/internal/telemetry"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"go.uber.org/zap"
)

type IngressEgress int

const (
	Ingress IngressEgress = iota
	Egress
)

const (
	fidelityCheckFail = "fail"
	fidelityCheckPass = "pass"

	fidelityMetricResult    = "result"
	fidelityMetricSignal    = "signal"
	fidelityMetricAttribute = "attribute"
)

type collectorMetric string
type fidelityMetric string

const (
	logsAcceptedMetric    collectorMetric = "otelcol_receiver_accepted_log_records_total"
	logsSentMetric        collectorMetric = "otelcol_exporter_sent_log_records_total"
	metricsAcceptedMetric collectorMetric = "otelcol_receiver_accepted_metric_points_total"
	metricsSentMetric     collectorMetric = "otelcol_exporter_sent_metric_points_total"
	spansAcceptedMetric   collectorMetric = "otelcol_receiver_accepted_spans_total"
	spansSentMetric       collectorMetric = "otelcol_exporter_sent_spans_total"

	signalParityFidelityMetric    fidelityMetric = "mdai_fidelity_signal_checks_total"
	signalPolicyFidelityMetric    fidelityMetric = "mdai_fidelity_required_signal_checks_total"
	attributeParityFidelityMetric fidelityMetric = "mdai_fidelity_attribute_checks_total"
	attributePolicyFidelityMetric fidelityMetric = "mdai_fidelity_required_attribute_checks_total"
)

type ValidationResult struct {
	Parity     bool                 `json:"parity"`
	Policy     bool                 `json:"policy"`
	Attributes ValidationAttributes `json:"attributes"`
}

type ValidationAttributes struct {
	Parity map[string]bool `json:"parity"`
	Policy map[string]bool `json:"policy"`
}

// parsedSample holds strictly typed data extracted from a model.Sample
type parsedSample struct {
	Value     float64
	Signal    telemetry.MLT
	Result    string
	Attribute string
}

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

func (cs *ConnectionStatus) VerifyDataFidelity(ctx context.Context, connectionName string, telemetryTypes []telemetry.MLT) (bool, *octantv1alpha.ValidationResultsBySignal, error) {
	dataIntegrity := true

	attrParity, err := cs.checkAttributeFidelity(ctx, connectionName, telemetryTypes, attributeParityFidelityMetric)
	if err != nil {
		return false, nil, fmt.Errorf("checking attribute parity fidelity: %w", err)
	}

	attrPolicy, err := cs.checkAttributeFidelity(ctx, connectionName, telemetryTypes, attributePolicyFidelityMetric)
	if err != nil {
		return false, nil, fmt.Errorf("checking attribute policy fidelity: %w", err)
	}

	signalParity, err := cs.checkSignalFidelity(ctx, connectionName, telemetryTypes, signalParityFidelityMetric)
	if err != nil {
		return false, nil, fmt.Errorf("checking signal parity fidelity: %w", err)
	}

	signalPolicy, err := cs.checkSignalFidelity(ctx, connectionName, telemetryTypes, signalPolicyFidelityMetric)
	if err != nil {
		return false, nil, fmt.Errorf("checking signal policy fidelity: %w", err)
	}

	results := octantv1alpha.ValidationResultsBySignal{}
	for _, t := range telemetryTypes {
		res := octantv1alpha.ValidationResult{
			Parity: signalParity[t],
			Policy: signalPolicy[t],
			Attributes: &octantv1alpha.ValidationAttributes{
				Parity: attrParity[t],
				Policy: attrPolicy[t],
			},
		}

		if !res.Parity && !res.Policy {
			dataIntegrity = false
		}

		switch t {
		case telemetry.Logs:
			results.Logs = &res
		case telemetry.Metrics:
			results.Metrics = &res
		case telemetry.Traces:
			results.Traces = &res
		default:
			cs.logger.Warn("exotic telemetry type in VerifyDataFidelity (how did we get here?!)", zap.String("telemetryType", string(t)))
		}
	}

	return dataIntegrity, &results, nil
}

func (cs *ConnectionStatus) IsTelemetryFlowing(ctx context.Context, connectionName string, ie IngressEgress, telemetryTypes []telemetry.MLT) (bool, error) {
	for _, connectionType := range telemetryTypes {
		var promQuery string
		switch connectionType {
		case telemetry.Logs:
			promQuery = buildFlowQuery(connectionName, ie, telemetry.Logs)
		case telemetry.Traces:
			promQuery = buildFlowQuery(connectionName, ie, telemetry.Traces)
		case telemetry.Metrics:
			promQuery = buildFlowQuery(connectionName, ie, telemetry.Metrics)
		default:
			return false, fmt.Errorf("unknown telemetry type: %s", connectionType)
		}

		resultVector, err := cs.queryVector(ctx, promQuery)
		if err != nil {
			return false, fmt.Errorf("failed to query prometheus: %w", err)
		}

		if len(resultVector) == 0 {
			return false, nil
		}

		if float64(resultVector[0].Value) <= 0 {
			return false, nil
		}
	}
	return true, nil
}

func (cs *ConnectionStatus) checkAttributeFidelity(ctx context.Context, connectionName string, telemetryTypes []telemetry.MLT, metricName fidelityMetric) (map[telemetry.MLT]map[string]bool, error) {
	attrs := make(map[telemetry.MLT]map[string]bool)
	for _, t := range telemetryTypes {
		attrs[t] = make(map[string]bool)
	}

	query := buildValidationQuery(metricName, connectionName)
	vector, err := cs.queryVector(ctx, query)
	if err != nil {
		return nil, err
	}

	for _, sample := range vector {
		parsed, ok := parseFidelitySample(sample)
		if !ok || parsed.Attribute == "" {
			continue
		}

		targetMap, exists := attrs[parsed.Signal]
		if !exists {
			continue
		}

		// Only a strict "pass" can initialize the attribute as true,
		// and only if we haven't already marked it as false.
		if parsed.Result == fidelityCheckPass {
			if _, exists := targetMap[parsed.Attribute]; !exists {
				targetMap[parsed.Attribute] = true
			}
		} else {
			// "fail", "unknown", or any unexpected value explicitly marks it false.
			// This will overwrite a previously recorded "true" for this attribute.
			targetMap[parsed.Attribute] = false
		}
	}

	return attrs, nil
}

func (cs *ConnectionStatus) checkSignalFidelity(ctx context.Context, connectionName string, telemetryTypes []telemetry.MLT, metricName fidelityMetric) (map[telemetry.MLT]bool, error) {
	signals := make(map[telemetry.MLT]bool)
	failsSeen := make(map[telemetry.MLT]bool)

	for _, t := range telemetryTypes {
		signals[t] = false
		failsSeen[t] = false
	}

	query := buildValidationQuery(metricName, connectionName)
	vector, err := cs.queryVector(ctx, query)
	if err != nil {
		return nil, err
	}

	for _, sample := range vector {
		parsed, ok := parseFidelitySample(sample)
		if !ok {
			continue
		}

		if _, exists := signals[parsed.Signal]; !exists {
			continue
		}

		switch parsed.Result {
		case fidelityCheckFail:
			signals[parsed.Signal] = false
			failsSeen[parsed.Signal] = true
		case fidelityCheckPass:
			if !failsSeen[parsed.Signal] {
				signals[parsed.Signal] = true
			}
		default:
			cs.logger.Info(fmt.Sprintf("encountered unexpected fidelity check metric label %s=%q for metric name %s data type %s", fidelityMetricResult, parsed.Result, metricName, parsed.Signal))
		}
	}

	return signals, nil
}

// parseFidelitySample extracts strongly typed data from a PromQL sample.
// It returns false if the sample is mathematically insignificant (<= 0)
// or should otherwise be skipped.
func parseFidelitySample(sample *model.Sample) (parsedSample, bool) {
	val := float64(sample.Value)
	if val <= 0 {
		return parsedSample{}, false
	}

	return parsedSample{
		Value:     val,
		Signal:    telemetry.MLT(sample.Metric[fidelityMetricSignal]),
		Result:    string(sample.Metric[fidelityMetricResult]),
		Attribute: string(sample.Metric[fidelityMetricAttribute]),
	}, true
}

func (cs *ConnectionStatus) queryVector(ctx context.Context, query string) (model.Vector, error) {
	results, _, err := cs.promClient.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}

	resultVector, ok := results.(model.Vector)
	if !ok {
		if results == nil {
			return nil, nil
		}
		return nil, errors.New("failed to convert result to model.Vector")
	}

	return resultVector, nil
}

func buildFlowQuery(connectionName string, ingressEgress IngressEgress, telemetryType telemetry.MLT) string {
	return fmt.Sprintf(
		"increase(%s{%s=%q, mdai_connection=%q, service_name=%q}[10m])",
		ingressEgress.getCollectorMLTMetric(telemetryType),
		ingressEgress.getComponentType(),
		"datadog", connectionName,
		connectionName+"-collector",
	)
}

func buildValidationQuery(metricName fidelityMetric, connectionName string) string {
	return fmt.Sprintf(`increase(%s{mdai_connection="%s-telemetry-validation"}[10m])`, metricName, connectionName)
}

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

func (ie IngressEgress) getComponentType() string {
	if ie == Ingress {
		return "receiver"
	}
	return "exporter"
}
