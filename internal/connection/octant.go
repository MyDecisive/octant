package connection

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mydecisive/mdai-gateway/internal/metrics"
	"github.com/mydecisive/mdai-gateway/internal/telemetry"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type OctantConnectionData struct {
	SourceType     string          `json:"sourceType"`
	TelemetryTypes []telemetry.MLT `json:"telemetryTypes"`
	Deployment     *Deployment     `json:"deployment,omitempty"`
}

type Deployment struct {
	Type   string         `json:"type"`
	Fields map[string]any `json:"fields"`
}

type ArgoDeployment struct {
	Branch string `json:"branch"`
}

var _ Connection[OctantConnectionData] = (*OctantConnection)(nil)

type OctantConnection struct {
	k8sClient         kubernetes.Interface
	logger            *zap.Logger
	connectionMetrics *metrics.ConnectionStatus
}

func NewOctantConnection(k8sClient kubernetes.Interface, promClient promv1.API, logger *zap.Logger) *OctantConnection {
	return &OctantConnection{
		k8sClient:         k8sClient,
		logger:            logger,
		connectionMetrics: metrics.NewConnectionStatus(promClient, logger),
	}
}

func (oc *OctantConnection) GetConnectionStatus(ctx context.Context, namespace, connectionName string) (*Status, error) {
	var (
		receivingData bool
		sendingData   bool
		dataIntegrity bool
	)
	connection, err := oc.GetConnectionByName(ctx, namespace, connectionName)
	if err != nil {
		return nil, fmt.Errorf("getting connection: %w", err)
	}

	// for each telemetry type on the connection, check for increasing metrics on the receiver (receiving data)
	receivingData, err = oc.connectionMetrics.IsTelemetryFlowing(ctx, metrics.Ingress, connection.TelemetryTypes)
	if err != nil {
		return nil, fmt.Errorf("querying telemetry ingress status: %w", err)
	}

	// for each telemetry type on the connection, check for increasing metrics on the exporter (sending data)
	sendingData, err = oc.connectionMetrics.IsTelemetryFlowing(ctx, metrics.Egress, connection.TelemetryTypes)
	if err != nil {
		return nil, fmt.Errorf("querying telemetry egress status: %w", err)
	}

	dataIntegrity, err = oc.connectionMetrics.VerifyDataFidelity(ctx, connection.TelemetryTypes)
	if err != nil {
		return nil, fmt.Errorf("verifying data integrity: %w", err)
	}

	return &Status{
		ReceivingData: receivingData,
		SendingData:   sendingData,
		DataIntegrity: dataIntegrity,
	}, nil
}

func (oc *OctantConnection) GetConnectionByName(ctx context.Context, namespace, name string) (*OctantConnectionData, error) {
	configmap, err := oc.k8sClient.CoreV1().ConfigMaps(namespace).Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil // nolint: nilnil
		}
		return nil, fmt.Errorf("failed to get configmap %s: %w", connectionsConfigmapName, err)
	}

	if _, ok := configmap.Data[name]; !ok {
		return nil, nil // nolint: nilnil
	}

	var connection OctantConnectionData
	if err = json.Unmarshal([]byte(configmap.Data[name]), &connection); err != nil {
		return nil, fmt.Errorf("failed to unmarshal connection data: %w", err)
	}
	return &connection, nil
}

func (oc *OctantConnection) SaveConnection(ctx context.Context, connection OctantConnectionData, namespace, connectionName string) error {
	jsonData, err := json.Marshal(connection)
	if err != nil {
		return fmt.Errorf("failed to marshal connection data: %w", err)
	}

	cm, err := oc.k8sClient.CoreV1().ConfigMaps(namespace).Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Create the confmap if it does not exist
			return createConnectionConfigMap(ctx, oc.k8sClient, namespace, connectionsConfigmapName, connectionName, string(jsonData))
		}
		return fmt.Errorf("failed to fetch configmap %s: %w", connectionsConfigmapName, err)
	}
	// Update the confmap if it already exists
	return updateConfigMapWithConnection(ctx, oc.k8sClient, namespace, cm, connectionName, string(jsonData))
}

func (oc *OctantConnection) DeleteConnection(ctx context.Context, namespace, connectionName string) error {
	cm, err := oc.k8sClient.CoreV1().ConfigMaps(namespace).Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to fetch configmap %s: %w", connectionsConfigmapName, err)
	}

	if cm.Data == nil {
		return nil
	}
	if _, exists := cm.Data[connectionName]; !exists {
		return nil
	}

	delete(cm.Data, connectionName)

	if _, err = oc.k8sClient.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update configmap %s after deletion: %w", connectionsConfigmapName, err)
	}

	return nil
}
