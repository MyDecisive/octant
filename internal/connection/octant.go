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
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/metrics"
	"github.com/mydecisive/octant/internal/telemetry"
	"go.uber.org/zap"
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
	MdaiNamespace  string                        `json:"mdaiNamespace"`
}

type OctantConnection struct {
	k8sClient          kubernetes.Interface
	argoIntegration    integration.Integration[integration.ArgoCDIntegrationData]
	datadogIntegration integration.Integration[integration.DataDogIntegrationData]
	connectionMetrics  metrics.ConnectionStatus
	configuration      *config.Configuration
	argoClient         argocd.APIClient
	generator          ManifestGenerator
}

func NewOctantConnection(
	k8sClient kubernetes.Interface,
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData],
	datadogIntegration integration.Integration[integration.DataDogIntegrationData],
	connectionMetrics metrics.ConnectionStatus,
	configuration *config.Configuration,
	argoClient argocd.APIClient,
	generator ManifestGenerator,
) *OctantConnection {
	return &OctantConnection{
		k8sClient:          k8sClient,
		argoIntegration:    argoIntegration,
		datadogIntegration: datadogIntegration,
		connectionMetrics:  connectionMetrics,
		configuration:      configuration,
		argoClient:         argoClient,
		generator:          generator,
	}
}

var _ Connection[OctantConnectionData] = (*OctantConnection)(nil)

func (oc *OctantConnection) GetConnectionStatus(
	ctx context.Context,
	input ConnectionCRUDInput,
	validatorRunID string,
) (
	*octantv1alpha.GetConnectionStatusResponse,
	error,
) {
	connection, err := oc.GetConnectionByName(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("getting connection: %w", err)
	}
	if connection == nil {
		return nil, fmt.Errorf("connection '%s' not found in namespace '%s'", input.ConnectionName, input.Namespace)
	}

	return oc.connectionMetrics.GetConnectionStatus(
		ctx,
		input.Namespace,
		input.ConnectionName,
		connection.TelemetryTypes,
		validatorRunID,
	)
}

func (oc *OctantConnection) GetConnectionByName(
	ctx context.Context,
	input ConnectionCRUDInput,
) (*OctantConnectionData, error) {
	configmap, err := oc.k8sClient.CoreV1().
		ConfigMaps(oc.configuration.CurrentNamespace).
		Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			input.Logger.Warn("configmap not found", zap.String("configmap", connectionsConfigmapName))
			return nil, nil // nolint: nilnil
		}
		return nil, fmt.Errorf("failed to get configmap %s: %w", connectionsConfigmapName, err)
	}

	if _, ok := configmap.Data[input.ConnectionName]; !ok {
		return nil, fmt.Errorf("connection '%s' not found", input.ConnectionName)
	}

	var connection OctantConnectionData
	if err = json.Unmarshal([]byte(configmap.Data[input.ConnectionName]), &connection); err != nil {
		return nil, fmt.Errorf("failed to unmarshal connection data: %w", err)
	}

	return &connection, nil
}

func (oc *OctantConnection) DeleteConnection(ctx context.Context, input ConnectionCRUDInput) error {
	cm, getCMErr := oc.k8sClient.CoreV1().
		ConfigMaps(oc.configuration.CurrentNamespace).
		Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if getCMErr != nil {
		input.Logger.Warn("configmap not found", zap.String("configmap", connectionsConfigmapName))
		return fmt.Errorf("failed to fetch configmap %s: %w", connectionsConfigmapName, getCMErr)
	}

	if _, exists := cm.Data[input.ConnectionName]; !exists {
		input.Logger.Warn("connection not found", zap.String("connectionName", input.ConnectionName))
		return fmt.Errorf("connection '%s' not found", input.ConnectionName)
	}

	var connection OctantConnectionData
	if err := json.Unmarshal([]byte(cm.Data[input.ConnectionName]), &connection); err != nil {
		return fmt.Errorf("failed to unmarshal connection data: %w", err)
	}

	// TODO: This should be refactored to a more robust deployment-based task system
	if connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		if deleteErr := oc.deleteArgoApp(ctx, input.Logger, input.ConnectionName, connection); deleteErr != nil {
			return deleteErr
		}
	}

	delete(cm.Data, input.ConnectionName)

	if _, err := oc.k8sClient.CoreV1().
		ConfigMaps(oc.configuration.CurrentNamespace).
		Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update configmap %s after deletion: %w", connectionsConfigmapName, err)
	}

	return nil
}

func (oc *OctantConnection) GetConnections(ctx context.Context, input ConnectionCRUDInput) ([]string, error) {
	configmap, err := oc.k8sClient.CoreV1().
		ConfigMaps(oc.configuration.CurrentNamespace).
		Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if err != nil {
		input.Logger.Warn("configmap not found", zap.String("configmap", connectionsConfigmapName))
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
	input ConnectionCRUDInput,
) ([]string, error) {
	return oc.connectionMetrics.GetConnectionValidatorRuns(ctx, input.Namespace, input.ConnectionName)
}

func (oc *OctantConnection) SaveConnection(
	ctx context.Context,
	connection OctantConnectionData,
	input ConnectionCRUDInput,
) error {
	if !input.Skip {
		if err := oc.createOrUpdate(ctx, connection, input); err != nil {
			return fmt.Errorf("%w:%w", ErrSave, err)
		}
	}

	if connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		err := oc.sideloadConnectionApp(ctx, input.Logger, input.ConnectionName, connection)
		if err != nil {
			return fmt.Errorf("%w:%w", ErrSideloadArgo, err)
		}
	}

	return nil
}

func (oc *OctantConnection) PutConnectionValidatorRun(ctx context.Context, input ConnectionCRUDInput) (string, error) {
	connection, err := oc.GetConnectionByName(ctx, input)
	if err != nil {
		return "", fmt.Errorf("getting connection: %w", err)
	}
	if connection == nil {
		return "", fmt.Errorf("connection '%s' not found", input.ConnectionName)
	}

	if connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		return oc.sideloadValidatorForConnection(ctx, input.Logger, input.ConnectionName, input.Namespace)
	}

	return "", nil
}

func (oc *OctantConnection) DeleteConnectionValidator(ctx context.Context, input ConnectionCRUDInput) error {
	cm, getCMErr := oc.k8sClient.CoreV1().
		ConfigMaps(oc.configuration.CurrentNamespace).
		Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if getCMErr != nil {
		input.Logger.Warn("fetching connection configmap", zap.Error(getCMErr))
		return fmt.Errorf("failed to fetch configmap %s: %w", connectionsConfigmapName, getCMErr)
	}

	if _, exists := cm.Data[input.ConnectionName]; !exists {
		input.Logger.Warn("connection not found in configmap", zap.String("connectionName", input.ConnectionName))
		return fmt.Errorf("connection not found in configmap %s", input.ConnectionName)
	}

	var connection OctantConnectionData
	if err := json.Unmarshal([]byte(cm.Data[input.ConnectionName]), &connection); err != nil {
		return fmt.Errorf("failed to unmarshal connection data: %w", err)
	}

	// TODO: This should be refactored to a more robust deployment-based task system
	if connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		templateData, err := oc.createTemplateData(ctx, input.ConnectionName, connection)
		if err != nil {
			return err
		}
		return oc.deleteValidatorResource(ctx, input.Logger, input.ConnectionName, templateData)
	}

	return nil
}

// createOrUpdate creates a connection if it doesn't already exist;
// Otherwise, this will update the existing connection.
func (oc *OctantConnection) createOrUpdate(
	ctx context.Context,
	connection OctantConnectionData,
	input ConnectionCRUDInput,
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

	cm, err := oc.k8sClient.CoreV1().
		ConfigMaps(oc.configuration.CurrentNamespace).
		Get(ctx, connectionsConfigmapName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to fetch configmap %s: %w", connectionsConfigmapName, err)
		}
		if createErr := oc.createConnection(
			ctx,
			connection,
			oc.configuration.CurrentNamespace,
			input.ConnectionName,
		); createErr != nil {
			return createErr
		}
	} else {
		if updateErr := oc.updateConnection(
			ctx,
			cm,
			connection,
			oc.configuration.CurrentNamespace,
			input.ConnectionName,
		); updateErr != nil {
			return updateErr
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
