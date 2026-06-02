package connection

import (
	"context"
	_ "embed" // nolint: revive
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mydecisive/mdai-data-core/kube"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/telemetry"
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
	cmStore            kube.ConfigMapStore
	argoIntegration    integration.Integration[integration.ArgoCDIntegrationData]
	datadogIntegration integration.Integration[integration.DataDogIntegrationData]
	configuration      *config.Configuration
	argoClient         argocd.APIClient
	generator          ManifestGenerator
}

// OctantConnectionOption is a dependency option to provide to a new OctantConnection.
type OctantConnectionOption func(*OctantConnection)

// NewOctantConnection creates and returns a new OctantConnection.
func NewOctantConnection(
	configMapStore kube.ConfigMapStore, // required
	configuration *config.Configuration, // required
	options ...OctantConnectionOption,
) *OctantConnection {
	oc := &OctantConnection{
		cmStore:       configMapStore,
		configuration: configuration,
	}

	for _, option := range options {
		option(oc)
	}
	return oc
}

// WithArgoCDIntegration provides an argocd integration to the octant connection.
func WithArgoCDIntegration(
	theIntegration integration.Integration[integration.ArgoCDIntegrationData],
) OctantConnectionOption {
	return func(o *OctantConnection) {
		o.argoIntegration = theIntegration
	}
}

// WithDatadogIntegration provides a datadog integration to the octant connection.
func WithDatadogIntegration(
	theIntegration integration.Integration[integration.DataDogIntegrationData],
) OctantConnectionOption {
	return func(o *OctantConnection) {
		o.datadogIntegration = theIntegration
	}
}

// WithArgoClient provides the argocd api client to the octant connection.
func WithArgoClient(client argocd.APIClient) OctantConnectionOption {
	return func(o *OctantConnection) {
		o.argoClient = client
	}
}

// WithGenerator provides a manifest generator to the octant connection.
func WithGenerator(generator ManifestGenerator) OctantConnectionOption {
	return func(o *OctantConnection) {
		o.generator = generator
	}
}

var _ Connection = (*OctantConnection)(nil)

func (oc *OctantConnection) GetConnectionByName(
	_ context.Context,
	input ConnectionCRUDInput,
) (*OctantConnectionData, error) {
	configmap, err := oc.cmStore.GetConfigmapByNameAndNamespace(
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
	cm, getCMErr := oc.cmStore.GetConfigmapByNameAndNamespace(
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
		if deleteErr := oc.deleteArgoApp(ctx, input.Logger, input.ConnectionName, connection); deleteErr != nil {
			return deleteErr
		}
	}

	delete(cm.Data, input.ConnectionName)

	return oc.cmStore.UpdateConfigMap(ctx, oc.configuration.CurrentNamespace, cm)
}

func (oc *OctantConnection) GetConnections(ctx context.Context, input ConnectionCRUDInput) ([]string, error) {
	configmap, err := oc.cmStore.GetConfigmapByNameAndNamespace(
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

func (oc *OctantConnection) SaveConnection(
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

	cm, err := oc.cmStore.GetConfigmapByNameAndNamespace(
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

	if connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		err = oc.sideloadConnectionApp(ctx, input.Logger, input.ConnectionName, connection)
		if err != nil {
			return err
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
		oc.cmStore,
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
		oc.cmStore,
		namespace,
		cm,
		connectionName,
		string(jsonData),
	)
}
