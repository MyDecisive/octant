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
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/metrics"
	"github.com/mydecisive/octant/internal/telemetry"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
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
	configMapStore     kube.ConfigMapStore
	secretStore        kube.SecretStore
	k8sClient          kubernetes.Interface
	argoIntegration    integration.Integration[integration.ArgoCDIntegrationData]
	datadogIntegration integration.Integration[integration.DataDogIntegrationData]
	connectionMetrics  metrics.ConnectionStatus
	configuration      *config.Configuration
	argoClient         argocd.APIClient
	generator          ManifestGenerator
}

// OctantConnectionOption is a dependency option to provide to a new OctantConnection.
type OctantConnectionOption func(*OctantConnection)

// NewOctantConnection creates and returns a new OctantConnection.
func NewOctantConnection(
	configMapStore kube.ConfigMapStore, // required
	secretStore kube.SecretStore,
	configuration *config.Configuration, // required
	options ...OctantConnectionOption,
) *OctantConnection {
	oc := &OctantConnection{
		configMapStore: configMapStore,
		secretStore:    secretStore,
		configuration:  configuration,
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

// WithK8sClient provides a kubernetes client to the octant connection.
func WithK8sClient(k8sClient kubernetes.Interface) OctantConnectionOption {
	return func(o *OctantConnection) {
		o.k8sClient = k8sClient
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

// WithConnectionMetrics provides connection metrics to the octant connection.
func WithConnectionMetrics(connectionMetrics metrics.ConnectionStatus) OctantConnectionOption {
	return func(o *OctantConnection) {
		o.connectionMetrics = connectionMetrics
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
		if deleteErr := oc.deleteArgoApp(ctx, input.Logger, input.ConnectionName, connection); deleteErr != nil {
			return deleteErr
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
		connection.Deployment != nil &&
		connection.Deployment.Type == ArgoSideloadDeploymentType {
		templateData, err := oc.createTemplateData(ctx, input.ConnectionName, connection)
		if err != nil {
			return err
		}

		// render and apply the integration secret and necessary rbac.
		if err = oc.applyConnectionSecret(ctx, templateData); err != nil {
			return err
		}
		if err = oc.sideloadConnectionApp(ctx, input.Logger, input.ConnectionName, templateData); err != nil {
			// TODO: revert secret??
			return err
		}
	}

	return nil
}

func (oc *OctantConnection) applyConnectionSecret(ctx context.Context, templateData *ArgoConnectionTemplateData) error {
	secretManifest, err := oc.generator.RenderConnectionSecret(templateData, YAMLOutputFormat)
	if err != nil {
		return fmt.Errorf("rendering secret template: %w", err)
	}

	var secret corev1.Secret
	if err = yaml.Unmarshal(secretManifest, &secret); err != nil {
		return fmt.Errorf("unmarshal secret template: %w", err)
	}

	if err = oc.secretStore.CreateSecret(ctx, templateData.Namespace, &secret); err != nil {
		return fmt.Errorf("creating secret: %w", err)
	}

	// render and apply the role for the secret.
	secretRoleManifest, err := oc.generator.RenderConnectionSecretRole(templateData, YAMLOutputFormat)
	if err != nil {
		return fmt.Errorf("rendering secret role template: %w", err)
	}

	var secretRole v1.Role
	if err = yaml.Unmarshal(secretRoleManifest, &secretRole); err != nil {
		return fmt.Errorf("unmarshal secret role template: %w", err)
	}

	if _, err = oc.k8sClient.RbacV1().Roles(templateData.Namespace).Create(ctx, &secretRole, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("creating role: %w", err)
	}

	// render and apply the role binding that binds the role to the service account for the secret.
	secretRoleBindingManifest, err := oc.generator.RenderConnectionSecretRoleBinding(templateData, YAMLOutputFormat)
	if err != nil {
		return fmt.Errorf("rendering secret role binding template: %w", err)
	}

	var secretRoleBinding v1.RoleBinding
	if err = yaml.Unmarshal(secretRoleBindingManifest, &secretRole); err != nil {
		return fmt.Errorf("unmarshal secret role binding template: %w", err)
	}

	if _, err = oc.k8sClient.RbacV1().RoleBindings(templateData.Namespace).Create(ctx, &secretRoleBinding, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("creating role binding: %w", err)
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
