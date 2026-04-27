package connection

import (
	"context"
	_ "embed" // nolint: revive
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/metrics"
	"github.com/mydecisive/octant/internal/telemetry"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type OctantConnectionDestination struct {
	DestinationType string `json:"type"`
	IntegrationName string `json:"integrationName"`
}

type OctantConnectionData struct {
	SourceType     string                        `json:"sourceType"`
	Destinations   []OctantConnectionDestination `json:"destinations"`
	TelemetryTypes []telemetry.MLT               `json:"telemetryTypes"`
	Deployment     *Deployment                   `json:"deployment,omitempty"`
	Status         any                           `json:"status,omitempty"`
}

type OctantConnection struct {
	httpClient        *http.Client
	k8sClient         kubernetes.Interface
	argoClient        integration.Integration[integration.ArgoCDIntegrationData]
	datadogClient     integration.Integration[integration.DataDogIntegrationData]
	PrometheusClient  promv1.API
	logger            *zap.Logger
	connectionMetrics *metrics.ConnectionStatus
	// TODO: Refactor connection operations to use tasksets/plans instead of if-argo-then
	// taskSets      map[DeploymentType]DeploymentTaskSet
}

func NewOctantConnection(
	httpClient *http.Client,
	k8sClient kubernetes.Interface,
	argoClient integration.Integration[integration.ArgoCDIntegrationData],
	datadogClient integration.Integration[integration.DataDogIntegrationData],
	promClient promv1.API,
	logger *zap.Logger,
) *OctantConnection {
	return &OctantConnection{
		httpClient:        httpClient,
		k8sClient:         k8sClient,
		argoClient:        argoClient,
		datadogClient:     datadogClient,
		logger:            logger,
		connectionMetrics: metrics.NewConnectionStatus(promClient, logger),
	}
}

var _ Connection[OctantConnectionData] = (*OctantConnection)(nil)

func (oc *OctantConnection) GetConnectionStatus(
	ctx context.Context,
	namespace, connectionName string,
) (*Status, error) {
	var (
		receivingData bool
		sendingData   bool
	)
	connection, err := oc.GetConnectionByName(ctx, namespace, connectionName)
	if err != nil {
		return nil, fmt.Errorf("getting connection: %w", err)
	}
	if connection == nil {
		return nil, fmt.Errorf("connection '%s' not found in namespace '%s'", connectionName, namespace)
	}

	receivingData, err = oc.connectionMetrics.
		IsTelemetryFlowing(ctx, connectionName, metrics.Ingress, connection.TelemetryTypes)
	if err != nil {
		return nil, fmt.Errorf("querying telemetry ingress status: %w", err)
	}

	sendingData, err = oc.connectionMetrics.
		IsTelemetryFlowing(ctx, connectionName, metrics.Egress, connection.TelemetryTypes)
	if err != nil {
		return nil, fmt.Errorf("querying telemetry egress status: %w", err)
	}

	dataIntegrity, validationResults, err := oc.connectionMetrics.
		VerifyDataFidelity(ctx, connectionName, connection.TelemetryTypes)
	if err != nil {
		return nil, fmt.Errorf("verifying data integrity: %w", err)
	}

	return &Status{
		ReceivingData:     receivingData,
		SendingData:       sendingData,
		DataIntegrity:     dataIntegrity,
		ValidationResults: validationResults,
	}, nil
}

func (oc *OctantConnection) GetConnectionByName(
	ctx context.Context,
	namespace, name string,
) (*OctantConnectionData, error) {
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

	// TODO: This should be refactored to a more robust deployment-based task system
	if connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		argoApp, err := oc.getArgoAppStatus(ctx, name, namespace, connection)
		if err != nil {
			return &connection, err
		}

		connection.Status = argoApp
	}

	return &connection, nil
}

func (oc *OctantConnection) SaveConnection(
	ctx context.Context,
	connection OctantConnectionData,
	namespace, connectionName string,
) error {
	if connection.Deployment == nil {
		return errors.New("no deployment object found on octant connection; unable to create connection")
	}

	if !slices.Contains(
		[]DeploymentType{ArgoManifestsDeploymentType, ArgoSideloadDeploymentType},
		connection.Deployment.Type,
	) {
		return fmt.Errorf("invalid deployment type: %s", connection.Deployment.Type)
	}
	jsonData, err := json.Marshal(connection)
	if err != nil {
		return fmt.Errorf("failed to marshal connection data: %w", err)
	}

	cm, err := oc.k8sClient.CoreV1().ConfigMaps(namespace).Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to fetch configmap %s: %w", connectionsConfigmapName, err)
		}
		// Create the confmap if it does not exist
		if createErr := createConnectionConfigMap(
			ctx,
			oc.k8sClient,
			namespace,
			connectionsConfigmapName,
			connectionName,
			string(jsonData),
		); createErr != nil {
			return createErr
		}
	} else {
		// Update the confmap if it already exists
		if updateErr := updateConfigMapWithConnection(
			ctx,
			oc.k8sClient,
			namespace,
			cm,
			connectionName,
			string(jsonData),
		); updateErr != nil {
			return updateErr
		}
	}

	// TODO: This should be refactored to a more robust deployment-based task system
	if connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		err := oc.pushArgoApp(ctx, namespace, connectionName, connection)
		if err != nil {
			return err
		}
	}

	return nil
}

func (oc *OctantConnection) DeleteConnection(ctx context.Context, namespace, connectionName string) error {
	cm, getCMErr := oc.k8sClient.CoreV1().ConfigMaps(namespace).Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if getCMErr != nil {
		if k8serrors.IsNotFound(getCMErr) {
			return nil
		}
		return fmt.Errorf("failed to fetch configmap %s: %w", connectionsConfigmapName, getCMErr)
	}

	if cm.Data == nil {
		return nil
	}
	if _, exists := cm.Data[connectionName]; !exists {
		return nil
	}

	var connection OctantConnectionData
	if err := json.Unmarshal([]byte(cm.Data[connectionName]), &connection); err != nil {
		return fmt.Errorf("failed to unmarshal connection data: %w", err)
	}

	// TODO: This should be refactored to a more robust deployment-based task system
	if connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		if deleteErr := oc.deleteArgoApp(ctx, connectionName, namespace, connection); deleteErr != nil {
			return deleteErr
		}
	}

	delete(cm.Data, connectionName)

	if _, err := oc.k8sClient.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update configmap %s after deletion: %w", connectionsConfigmapName, err)
	}

	return nil
}
