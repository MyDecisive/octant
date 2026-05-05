package connection

import (
	"context"
	_ "embed" // nolint: revive
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/metrics"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/mydecisive/octant/internal/wrapper"
	corev1 "k8s.io/api/core/v1"
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
	Created        time.Time                     `json:"created"`
}

type OctantConnection struct {
	httpClient        wrapper.HTTPClient
	k8sClient         kubernetes.Interface
	argoClient        integration.Integration[integration.ArgoCDIntegrationData]
	datadogClient     integration.Integration[integration.DataDogIntegrationData]
	connectionMetrics metrics.ConnectionStatus
	configuration     *config.Configuration
	// TODO: Refactor connection operations to use tasksets/plans instead of if-argo-then
	// taskSets      map[DeploymentType]DeploymentTaskSet
}

func NewOctantConnection(
	httpClient wrapper.HTTPClient,
	k8sClient kubernetes.Interface,
	argoClient integration.Integration[integration.ArgoCDIntegrationData],
	datadogClient integration.Integration[integration.DataDogIntegrationData],
	connectionMetrics metrics.ConnectionStatus,
	configuration *config.Configuration,
) *OctantConnection {
	return &OctantConnection{
		httpClient:        httpClient,
		k8sClient:         k8sClient,
		argoClient:        argoClient,
		datadogClient:     datadogClient,
		connectionMetrics: connectionMetrics,
		configuration:     configuration,
	}
}

var _ Connection[OctantConnectionData] = (*OctantConnection)(nil)

func (oc *OctantConnection) GetConnectionStatus(
	ctx context.Context,
	namespace string,
	connectionName string,
	validatorRunID string,
) (
	*octantv1alpha.GetConnectionStatusResponse,
	error,
) {
	connection, err := oc.GetConnectionByName(ctx, namespace, connectionName)
	if err != nil {
		return nil, fmt.Errorf("getting connection: %w", err)
	}
	if connection == nil {
		return nil, fmt.Errorf("connection '%s' not found in namespace '%s'", connectionName, namespace)
	}

	return oc.connectionMetrics.GetConnectionStatus(
		ctx,
		namespace,
		connectionName,
		connection.TelemetryTypes,
		validatorRunID,
	)
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

	return &connection, nil
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
		if deleteErr := oc.deleteArgoApp(ctx, connectionName, connection); deleteErr != nil {
			return deleteErr
		}
	}

	delete(cm.Data, connectionName)

	if _, err := oc.k8sClient.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update configmap %s after deletion: %w", connectionsConfigmapName, err)
	}

	return nil
}

func (oc *OctantConnection) GetConnections(ctx context.Context, namespace string) ([]string, error) {
	configmap, err := oc.k8sClient.CoreV1().ConfigMaps(namespace).Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to get configmap %s: %w", connectionsConfigmapName, err)
	}

	var names []string
	for name := range configmap.Data {
		names = append(names, name)
	}
	return names, nil
}

func (oc *OctantConnection) GetConnectionValidatorRuns(
	ctx context.Context,
	namespace, connectionName string,
) ([]string, error) {
	return oc.connectionMetrics.GetConnectionValidatorRuns(ctx, namespace, connectionName)
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

	cm, err := oc.k8sClient.CoreV1().ConfigMaps(namespace).Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to fetch configmap %s: %w", connectionsConfigmapName, err)
		}
		if createErr := oc.createConnection(ctx, connection, namespace, connectionName); createErr != nil {
			return createErr
		}
	} else {
		if updateErr := oc.updateConnection(ctx, cm, connection, namespace, connectionName); updateErr != nil {
			return updateErr
		}
	}

	if connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		err = oc.sideloadConnectionApp(ctx, namespace, connectionName, connection)
		if err != nil {
			return err
		}
	}

	return nil
}

func (oc *OctantConnection) PutConnectionValidatorRun(
	ctx context.Context,
	namespace string,
	connectionName string,
) (string, error) {
	connection, err := oc.GetConnectionByName(ctx, namespace, connectionName)
	if err != nil {
		return "", fmt.Errorf("getting connection: %w", err)
	}
	if connection == nil {
		return "", fmt.Errorf("connection '%s' not found", connectionName)
	}

	if connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		return oc.sideloadValidatorForConnection(ctx, *connection, connectionName, namespace)
	}

	return "", nil
}

func (oc *OctantConnection) DeleteConnectionValidator(ctx context.Context, namespace, connectionName string) error {
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
		if deleteErr := oc.deleteValidatorResource(ctx, connectionName, namespace, connection); deleteErr != nil {
			return deleteErr
		}
	}

	return nil
}

// createConnection creates a new connection configmap.
func (oc *OctantConnection) createConnection(
	ctx context.Context,
	connection OctantConnectionData,
	namespace, connectionName string,
) error {
	connection.Created = time.Now()
	jsonData, err := json.Marshal(connection)
	if err != nil {
		return fmt.Errorf("failed to marshal connection data: %w", err)
	}
	return createConnectionConfigMap(
		ctx,
		oc.k8sClient,
		namespace,
		connectionsConfigmapName,
		connectionName,
		string(jsonData),
	)
}

// updateConnection updates the existing connection configmap with the new data.
func (oc *OctantConnection) updateConnection(
	ctx context.Context,
	cm *corev1.ConfigMap,
	connection OctantConnectionData,
	namespace, connectionName string,
) error {
	jsonData, err := json.Marshal(connection)
	if err != nil {
		return fmt.Errorf("failed to marshal connection data: %w", err)
	}
	return updateConfigMapWithConnection(
		ctx,
		oc.k8sClient,
		namespace,
		cm,
		connectionName,
		string(jsonData),
	)
}
