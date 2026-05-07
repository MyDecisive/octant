package metrics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
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

type (
	collectorMetric string
	fidelityMetric  string
)

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

// parsedSample holds strictly typed data extracted from a model.Sample.
type parsedSample struct {
	Value     float64
	Signal    telemetry.MLT
	Result    string
	Attribute string
}

type ConnectionStatus interface {
	GetConnectionStatus(
		ctx context.Context,
		namespace string,
		connectionName string,
		telemetryTypes []telemetry.MLT,
		validatorRunID string,
	) (*octantv1alpha.GetConnectionStatusResponse, error)
	GetConnectionValidatorRuns(
		ctx context.Context,
		namespace string,
		connectionName string,
	) ([]string, error)
}

type PrometheusConnectionStatus struct {
	promClientFactory PromClientFactory
}

func NewPrometheusConnectionStatus(promClientFactory PromClientFactory) *PrometheusConnectionStatus {
	return &PrometheusConnectionStatus{
		promClientFactory: promClientFactory,
	}
}

// GetConnectionStatus reads OTEL Collector and validator metrics to ensure a MDAI Connection is working.
func (cs *PrometheusConnectionStatus) GetConnectionStatus(
	ctx context.Context,
	namespace string,
	connectionName string,
	telemetryTypes []telemetry.MLT,
	validatorRunID string,
) (*octantv1alpha.GetConnectionStatusResponse, error) {
	promClient, err := cs.promClientFactory.GetPromClient(namespace)
	if err != nil {
		return nil, err
	}

	receivingData, err := isTelemetryFlowing(ctx, promClient, connectionName, Ingress, telemetryTypes)
	if err != nil {
		return nil, fmt.Errorf("querying telemetry ingress status: %w", err)
	}

	sendingData, err := isTelemetryFlowing(ctx, promClient, connectionName, Egress, telemetryTypes)
	if err != nil {
		return nil, fmt.Errorf("querying telemetry egress status: %w", err)
	}

	dataIntegrity, validationResults, err := verifyDataFidelity(
		ctx, promClient, connectionName, telemetryTypes, validatorRunID,
	)
	if err != nil {
		return nil, fmt.Errorf("verifying data integrity: %w", err)
	}

	clientsConnected, err := getClientsConnected(ctx, promClient, connectionName)
	if err != nil {
		return nil, fmt.Errorf("getting clients connected: %w", err)
	}

	return &octantv1alpha.GetConnectionStatusResponse{
		ReceivingData:     receivingData,
		SendingData:       sendingData,
		DataIntegrity:     dataIntegrity,
		ClientsConnected:  clientsConnected,
		ValidationResults: validationResults,
	}, nil
}

// getClientsConnected reads the envoy cluster metrics to ensure that clients are connecting.
func getClientsConnected(
	ctx context.Context,
	promClient promv1.API,
	connectionName string,
) (bool, error) {
	promQuery := buildConnectedClientsQuery(connectionName)

	resultVector, err := queryVector(ctx, promClient, promQuery)
	if err != nil {
		return false, fmt.Errorf("failed to query prometheus: %w", err)
	}

	if len(resultVector) == 0 {
		return false, nil
	}

	if float64(resultVector[0].Value) <= 0 {
		return false, nil
	}
	return true, nil
}

// verifyDataFidelity reads MDAI Validator metrics to ensure that data coming in is meaningfully similar
// to data going out of the connection.
func verifyDataFidelity(
	ctx context.Context,
	promClient promv1.API,
	connectionName string,
	telemetryTypes []telemetry.MLT,
	validatorRunID string,
) (bool, *octantv1alpha.ValidationResultsBySignal, error) {
	dataIntegrity := true

	attrParity, err := checkAttributeFidelity(
		ctx, promClient, connectionName, telemetryTypes, validatorRunID, attributeParityFidelityMetric,
	)
	if err != nil {
		return false, nil, fmt.Errorf("checking attribute parity fidelity: %w", err)
	}

	attrPolicy, err := checkAttributeFidelity(
		ctx, promClient, connectionName, telemetryTypes, validatorRunID, attributePolicyFidelityMetric,
	)
	if err != nil {
		return false, nil, fmt.Errorf("checking attribute policy fidelity: %w", err)
	}

	signalParity, err := checkSignalFidelity(
		ctx, promClient, connectionName, telemetryTypes, validatorRunID, signalParityFidelityMetric,
	)
	if err != nil {
		return false, nil, fmt.Errorf("checking signal parity fidelity: %w", err)
	}

	signalPolicy, err := checkSignalFidelity(
		ctx, promClient, connectionName, telemetryTypes, validatorRunID, signalPolicyFidelityMetric,
	)
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

		if !res.GetParity() && !res.GetPolicy() {
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
			zap.L().Error(
				"exotic telemetry type in verifyDataFidelity (how did we get here?!)",
				zap.String("telemetryType", string(t)),
			)
		}
	}

	return dataIntegrity, &results, nil
}

// isTelemetryFlowing reads OTEL Collector metrics to ensure that the connection collector is
// receiving/sending telemetry.
func isTelemetryFlowing(
	ctx context.Context,
	promClient promv1.API,
	connectionName string,
	ie IngressEgress,
	telemetryTypes []telemetry.MLT,
) (bool, error) {
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

		resultVector, err := queryVector(ctx, promClient, promQuery)
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

// checkAttributeFidelity inspects attribute fidelity metric results and assigns a true/false
// for pass/fail per attribute.
func checkAttributeFidelity(
	ctx context.Context,
	promClient promv1.API,
	connectionName string,
	telemetryTypes []telemetry.MLT,
	validatorRunID string,
	metricName fidelityMetric,
) (map[telemetry.MLT]map[string]bool, error) {
	attrs := make(map[telemetry.MLT]map[string]bool)
	for _, t := range telemetryTypes {
		attrs[t] = make(map[string]bool)
	}

	query := buildValidationQuery(metricName, connectionName, validatorRunID)
	vector, err := queryVector(ctx, promClient, query)
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

// checkSignalFidelity inspects signal fidelity metric results and assigns a true/false for
// pass/fail per MLT.
func checkSignalFidelity(
	ctx context.Context,
	promClient promv1.API,
	connectionName string,
	telemetryTypes []telemetry.MLT,
	validatorRunID string,
	metricName fidelityMetric,
) (map[telemetry.MLT]bool, error) {
	signals := make(map[telemetry.MLT]bool)
	failsSeen := make(map[telemetry.MLT]bool)

	for _, t := range telemetryTypes {
		signals[t] = false
		failsSeen[t] = false
	}

	query := buildValidationQuery(metricName, connectionName, validatorRunID)
	vector, err := queryVector(ctx, promClient, query)
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
			zap.L().Info(fmt.Sprintf(
				"encountered unexpected fidelity check metric label %s=%q for metric name %s data type %s",
				fidelityMetricResult, parsed.Result, metricName, parsed.Signal,
			),
				zap.String("promLabel", fidelityMetricResult),
				zap.String("parsedResult", parsed.Result),
				zap.String("metric", string(metricName)),
				zap.String("telemetryType", string(parsed.Signal)))
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

// queryVector wraps a Prometheus query call to extract a well-typed vector model from the response.
func queryVector(ctx context.Context, promClient promv1.API, query string) (model.Vector, error) {
	results, _, err := promClient.Query(ctx, query, time.Now())
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

func buildValidationQuery(metricName fidelityMetric, connectionName string, validatorRunID string) string {
	return fmt.Sprintf(
		`increase(%s{mdai_connection="%s-telemetry-validation", telemetry_validation_run_id=%q}[10m])`,
		metricName,
		connectionName,
		validatorRunID,
	)
}

func buildConnectedClientsQuery(connectionName string) string {
	return fmt.Sprintf("increase(envoy_cluster_upstream_cx_total{mdai_connection=%q}[5m])", connectionName)
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

func (cs *PrometheusConnectionStatus) GetConnectionValidatorRuns(
	ctx context.Context,
	namespace string,
	connectionName string,
) ([]string, error) {
	promClient, err := cs.promClientFactory.GetPromClient(namespace)
	if err != nil {
		return nil, fmt.Errorf("getting prometheus client: %w", err)
	}

	allValidatorMetricsString := strings.Join(
		[]string{
			fmt.Sprintf("%s{mdai_connection=%q}", signalParityFidelityMetric, connectionName),
			fmt.Sprintf("%s{mdai_connection=%q}", signalPolicyFidelityMetric, connectionName),
			fmt.Sprintf("%s{mdai_connection=%q}", attributeParityFidelityMetric, connectionName),
			fmt.Sprintf("%s{mdai_connection=%q}", attributePolicyFidelityMetric, connectionName),
		}, " or ")
	query := fmt.Sprintf(`count by (telemetry_validation_run_id) (%s)`, allValidatorMetricsString)

	vector, err := queryVector(ctx, promClient, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query prometheus for validator runs: %w", err)
	}

	var runIDs []string
	seen := make(map[string]bool)

	for _, sample := range vector {
		runID := string(sample.Metric["telemetry_validation_run_id"])
		if runID != "" && !seen[runID] {
			seen[runID] = true
			runIDs = append(runIDs, runID)
		}
	}

	return runIDs, nil
}
