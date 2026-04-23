package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mydecisive/octant/internal/telemetry"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"go.uber.org/zap"
)

const (
	Ingress IngressEgress = iota
	Egress

	PolicyValidation ValidationType = "policyValidation"
	ParityValidation ValidationType = "parityValidation"

	fidelityCheckFail = "fail"
	fidelityCheckPass = "pass"

	fidelityMetricResult    = "result"
	fidelityMetricSignal    = "signal"
	fidelityMetricAttribute = "attribute"

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

var (
	signalParityMetricsToValidationType = map[fidelityMetric]ValidationType{
		signalParityFidelityMetric: ParityValidation,
		signalPolicyFidelityMetric: PolicyValidation,
	}
	attributeParityMetricsToValidationType = map[fidelityMetric]ValidationType{
		attributeParityFidelityMetric: ParityValidation,
		attributePolicyFidelityMetric: PolicyValidation,
	}
)

type IngressEgress int
type ValidationType string
type collectorMetric string
type fidelityMetric string
type SignalChecks map[ValidationType]bool

type ValidationResult struct {
	Parity     bool                 `json:"parity"`
	Policy     bool                 `json:"policy"`
	Attributes ValidationAttributes `json:"attributes"`
}

type ValidationAttributes struct {
	Parity map[string]bool `json:"parity"`
	Policy map[string]bool `json:"policy"`
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

func (cs *ConnectionStatus) VerifyDataFidelity(ctx context.Context, connectionName string, telemetryTypes []telemetry.MLT) (bool, map[telemetry.MLT]ValidationResult, error) {
	dataIntegrity := true

	attrData, err := cs.checkAttributeFidelity(ctx, connectionName, telemetryTypes)
	if err != nil {
		return false, nil, fmt.Errorf("checking attribute fidelity: %w", err)
	}

	signalData, err := cs.checkSignalFidelity(ctx, connectionName, telemetryTypes)
	if err != nil {
		return false, nil, fmt.Errorf("checking signal fidelity: %w", err)
	}

	results := make(map[telemetry.MLT]ValidationResult)
	for _, t := range telemetryTypes {
		res := ValidationResult{
			Parity:     signalData[t][ParityValidation],
			Policy:     signalData[t][PolicyValidation],
			Attributes: attrData[t],
		}

		if !res.Parity && !res.Policy {
			dataIntegrity = false
		}

		results[t] = res
	}

	return dataIntegrity, results, nil
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

func (cs *ConnectionStatus) checkAttributeFidelity(ctx context.Context, connectionName string, telemetryTypes []telemetry.MLT) (map[telemetry.MLT]ValidationAttributes, error) {
	attrs := makeEmptyAttributeValidationMap(telemetryTypes)

	for metricName, vType := range attributeParityMetricsToValidationType {
		query := buildValidationQuery(metricName, connectionName)
		vector, err := cs.queryVector(ctx, query)
		if err != nil {
			return nil, err
		}

		for _, sample := range vector {
			val := float64(sample.Value)
			if val <= 0 {
				continue
			}

			signal := telemetry.MLT(sample.Metric[fidelityMetricSignal])
			attrGroup, exists := attrs[signal]
			if !exists {
				continue
			}

			attrName := string(sample.Metric[fidelityMetricAttribute])
			result := string(sample.Metric[fidelityMetricResult])

			if attrName == "" {
				continue
			}

			targetMap := attrGroup.Parity
			if vType == PolicyValidation {
				targetMap = attrGroup.Policy
			}

			// Only a strict "pass" can initialize the attribute as true,
			// and only if we haven't already marked it as false.
			if result == fidelityCheckPass {
				if _, exists := targetMap[attrName]; !exists {
					targetMap[attrName] = true
				}
			} else {
				// "fail", "unknown", or any unexpected value explicitly marks it false.
				// This will overwrite a previously recorded "true" for this attribute.
				targetMap[attrName] = false
			}

			attrs[signal] = attrGroup
		}
	}

	return attrs, nil
}

func (cs *ConnectionStatus) checkSignalFidelity(ctx context.Context, connectionName string, telemetryTypes []telemetry.MLT) (map[telemetry.MLT]SignalChecks, error) {
	signals, failsSeen := makeEmptySignalCheckMaps(telemetryTypes)

	for metricName, vType := range signalParityMetricsToValidationType {
		query := buildValidationQuery(metricName, connectionName)
		vector, err := cs.queryVector(ctx, query)
		if err != nil {
			return nil, err
		}

		for _, sample := range vector {
			val := float64(sample.Value)
			if val <= 0 {
				continue
			}

			signal := telemetry.MLT(sample.Metric[fidelityMetricSignal])
			if _, exists := signals[signal]; !exists {
				continue
			}

			result := string(sample.Metric[fidelityMetricResult])
			switch result {
			case fidelityCheckFail:
				signals[signal][vType] = false
				failsSeen[signal][vType] = true
			case fidelityCheckPass:
				if !failsSeen[signal][vType] {
					signals[signal][vType] = true
				}
			default:
				cs.logger.Info(fmt.Sprintf("encountered unexpected fidelity check metric label %s=%q for metric name %s data type %s", fidelityMetricResult, result, metricName, signal))
			}
		}
	}

	return signals, nil
}

func makeEmptyAttributeValidationMap(telemetryTypes []telemetry.MLT) map[telemetry.MLT]ValidationAttributes {
	attrs := make(map[telemetry.MLT]ValidationAttributes)
	for _, t := range telemetryTypes {
		attrs[t] = ValidationAttributes{
			Parity: make(map[string]bool),
			Policy: make(map[string]bool),
		}
	}
	return attrs
}

func makeEmptySignalCheckMaps(telemetryTypes []telemetry.MLT) (map[telemetry.MLT]SignalChecks, map[telemetry.MLT]SignalChecks) {
	signals := make(map[telemetry.MLT]SignalChecks)
	failsSeen := make(map[telemetry.MLT]SignalChecks)

	for _, t := range telemetryTypes {
		signals[t] = SignalChecks{
			ParityValidation: false,
			PolicyValidation: false,
		}
		failsSeen[t] = SignalChecks{
			ParityValidation: false,
			PolicyValidation: false,
		}
	}
	return signals, failsSeen
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
