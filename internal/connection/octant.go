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
	"github.com/mydecisive/mdai-data-core/kube"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection/manifest"
	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	"github.com/mydecisive/octant/internal/metrics"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/samber/lo"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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

// OctantConnection encapsulates connection logic for the octant application.
type OctantConnection struct {
	configMapStore    kube.ConfigMapStore
	connectionMetrics metrics.ConnectionStatus
	configuration     *config.Configuration
	manifestManager   manifest.Manager
}

// NewOctantConnection creates and returns a new OctantConnection.
func NewOctantConnection(
	configMapStore kube.ConfigMapStore,
	configuration *config.Configuration,
	connectionMetrics metrics.ConnectionStatus,
	manifestManager manifest.Manager,
) *OctantConnection {
	return &OctantConnection{
		configMapStore:    configMapStore,
		configuration:     configuration,
		connectionMetrics: connectionMetrics,
		manifestManager:   manifestManager,
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
	_ context.Context,
	input ConnectionCRUDInput,
) (*OctantConnectionData, error) {
	configmap, err := oc.configMapStore.GetConfigmapByNameAndNamespace(
		connectionsConfigmapName,
		oc.configuration.CurrentNamespace,
	)
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
	cm, getCMErr := oc.configMapStore.GetConfigmapByNameAndNamespace(
		connectionsConfigmapName,
		oc.configuration.CurrentNamespace,
	)
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
		if deleteErr := oc.manifestManager.Unload(
			ctx,
			manifest.ManagerInput{
				Logger:                    input.Logger,
				DeploymentIntegrationName: connection.Deployment.IntegrationName,
				ConnectionName:            input.ConnectionName,
			},
			[]manifestdata.App{manifestdata.CONNECTION, manifestdata.VALIDATOR},
		); deleteErr != nil {
			return fmt.Errorf("remove app:%w", deleteErr)
		}
	}

	delete(cm.Data, input.ConnectionName)

	return oc.configMapStore.UpdateConfigMap(ctx, oc.configuration.CurrentNamespace, cm)
}

func (oc *OctantConnection) GetConnections(ctx context.Context, input ConnectionCRUDInput) ([]string, error) {
	configmap, err := oc.configMapStore.GetConfigmapByNameAndNamespace(
		connectionsConfigmapName,
		oc.configuration.CurrentNamespace,
	)
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
	if !input.OnlyDeploy {
		if err := oc.createOrUpdate(ctx, connection, input); err != nil {
			return err
		}
	}

	if !input.NoDeploy &&
		connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		if len(connection.Destinations) > 1 {
			return errors.New("multiple destination")
		}
		destination := lo.FilterMap(
			connection.Destinations,
			func(item OctantConnectionDestination, _ int) (manifestdata.Destination, bool) {
				return manifestdata.Destination{
					Type:            manifestdata.DATADOG, // TODO: Default to datadog cause we only allow datadog for now
					IntegrationName: item.IntegrationName,
				}, item.DestinationType == "datadog"
			})
		if len(destination) < 1 {
			return errors.New("unrecognized destination")
		}

		if err := oc.manifestManager.LoadConnection(ctx, input.Logger, manifestdata.ConnectionInput{
			ConnectionName:            input.ConnectionName,
			DeploymentIntegrationName: connection.Deployment.IntegrationName,
			Namespace:                 input.Namespace,
			TelemetryTypes:            connection.TelemetryTypes,
			Destinations:              destination,
		}); err != nil {
			return fmt.Errorf("install connection app:%w", err)
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
		runID := time.Now().UTC().Format(metrics.ValidatorRunIDFormat)
		if err := oc.manifestManager.LoadValidator(ctx, input.Logger, manifestdata.ValidatorInput{
			ConnectionName:            input.ConnectionName,
			DeploymentIntegrationName: connection.Deployment.IntegrationName,
			Namespace:                 input.Namespace,
			RunID:                     runID,
		}); err != nil {
			return "", fmt.Errorf("install validator app:%w", err)
		}
		return runID, nil
	}

	return "", nil
}

func (oc *OctantConnection) DeleteConnectionValidator(ctx context.Context, input ConnectionCRUDInput) error {
	cm, getCMErr := oc.configMapStore.GetConfigmapByNameAndNamespace(
		connectionsConfigmapName,
		oc.configuration.CurrentNamespace,
	)
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
		if deleteErr := oc.manifestManager.Unload(
			ctx,
			manifest.ManagerInput{
				Logger:                    input.Logger,
				DeploymentIntegrationName: connection.Deployment.IntegrationName,
				ConnectionName:            input.ConnectionName,
			},
			[]manifestdata.App{manifestdata.VALIDATOR},
		); deleteErr != nil {
			return fmt.Errorf("remove validator app:%w", deleteErr)
		}
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

	cm, err := oc.configMapStore.GetConfigmapByNameAndNamespace(
		connectionsConfigmapName,
		oc.configuration.CurrentNamespace,
	)
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
		oc.configMapStore,
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
	// ensure we're preserving the existing Created time.
	if _, ok := cm.Data[connectionName]; ok {
		var existingConnection OctantConnectionData
		if err := json.Unmarshal([]byte(cm.Data[connectionName]), &existingConnection); err != nil {
			return fmt.Errorf("failed to unmarshal connection data: %w", err)
		}
		connection.Created = existingConnection.Created
	}

	jsonData, err := json.Marshal(connection)
	if err != nil {
		return fmt.Errorf("failed to marshal connection data: %w", err)
	}
	return updateConfigMapWithConnection(
		ctx,
		oc.configMapStore,
		namespace,
		cm,
		connectionName,
		string(jsonData),
	)
}
