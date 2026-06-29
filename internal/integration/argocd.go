package integration

import (
	"context"
	"encoding/json"
	"fmt"
	v1 "github.com/mydecisive/octant/api/v1"
	"github.com/mydecisive/octant/internal/installlog"
	"go.uber.org/zap"

	"github.com/mydecisive/mdai-data-core/kube"
	"github.com/mydecisive/octant/internal/config"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

const argocdSecretName = "mdai-argocd-integration" // nolint: gosec

type ArgoCDIntegrationData struct {
	APIUrl       string `json:"apiUrl"`
	AccountToken string `json:"accountToken"`
}

type ArgoCDIntegration struct {
	secretStore     kube.SecretStore
	installLogStore installlog.InstallLogStore
	configuration   *config.Configuration
}

var _ Integration[ArgoCDIntegrationData] = (*ArgoCDIntegration)(nil)

// NewArgoCDIntegration returns a new instance of ArgoCDIntegration.
func NewArgoCDIntegration(
	secretStore kube.SecretStore,
	installLogStore installlog.InstallLogStore,
	configuration *config.Configuration,
) *ArgoCDIntegration {
	return &ArgoCDIntegration{
		secretStore:     secretStore,
		installLogStore: installLogStore,
		configuration:   configuration,
	}
}

// GetIntegrations retrieves any existing integrations
// in the provided namespace for the "mdai-argocd-integration" secret.
func (aci *ArgoCDIntegration) GetIntegrations(_ context.Context) (map[string]ArgoCDIntegrationData, error) {
	secret, err := aci.secretStore.GetSecretByNameAndNamespace(argocdSecretName, aci.configuration.CurrentNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil // nolint: nilnil
		}
		return nil, fmt.Errorf("failed to get secret %s: %w", argocdSecretName, err)
	}

	integrations := make(map[string]ArgoCDIntegrationData)
	for name, data := range secret.Data {
		var payload ArgoCDIntegrationData
		if unmarshalErr := json.Unmarshal(data, &payload); unmarshalErr != nil {
			continue // Skip invalid JSON entries
		}
		integrations[name] = payload
	}

	return integrations, nil
}

// GetIntegrationByName retrieves the existing
// integration in the provided namespace for the "mdai-argocd-integration" secret, if it exists.
func (aci *ArgoCDIntegration) GetIntegrationByName(
	ctx context.Context, name string,
) (*ArgoCDIntegrationData, error) {
	secret, err := aci.secretStore.GetSecretByNameAndNamespace(argocdSecretName, aci.configuration.CurrentNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil // nolint: nilnil
		}
		return nil, fmt.Errorf("failed to get secret %s: %w", argocdSecretName, err)
	}

	if _, ok := secret.Data[name]; !ok {
		return nil, fmt.Errorf("integration '%s' not found", name)
	}

	var payload ArgoCDIntegrationData
	if unmarshalErr := json.Unmarshal(secret.Data[name], &payload); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal integration data: %w", unmarshalErr)
	}
	return &payload, nil
}

// SetIntegration adds or updates the "mdai-argocd-integration" secret for the provided namespace.
func (aci *ArgoCDIntegration) SetIntegration(
	ctx context.Context,
	integrationName string,
	integrationData ArgoCDIntegrationData,
) error {
	jsonData, err := json.Marshal(integrationData)
	if err != nil {
		return fmt.Errorf("failed to marshal integration data: %w", err)
	}
	namespace := aci.configuration.CurrentNamespace
	secret, err := aci.secretStore.GetSecretByNameAndNamespace(argocdSecretName, aci.configuration.CurrentNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return aci.createSecretWithIntegration(ctx, integrationName, namespace, jsonData)
		}
		return fmt.Errorf("failed to fetch secret %s: %w", argocdSecretName, err)
	}
	// Update the secret if it already exists
	updateErr := updateSecretWithIntegration(ctx, aci.secretStore, namespace, integrationName, secret, jsonData)
	aci.writeInstallLogEntry(ctx, integrationName, namespace, updateErr)
	return updateErr
}

func (aci *ArgoCDIntegration) createSecretWithIntegration(ctx context.Context, integrationName string, namespace string, jsonData []byte) error {
	// Create the secret if it does not exist
	createErr := createIntegrationSecret(
		ctx,
		aci.secretStore,
		namespace,
		integrationName,
		argocdSecretName,
		kube.OctantIntegrationArgoType,
		jsonData,
	)
	aci.writeInstallLogEntry(ctx, integrationName, namespace, createErr)
	return createErr
}

func (aci *ArgoCDIntegration) writeInstallLogEntry(ctx context.Context, integrationName string, namespace string, createErr error) {
	result := v1.FailureOctantInstallEventResult
	if createErr == nil {
		result = v1.SuccessOctantInstallEventResult
	}
	if writeLogEntryErr := aci.installLogStore.AddInstallLogEvent(ctx, &v1.OctantInstallEvent{
		Action:    v1.CreateDeployIntegrationOctantInstallEventAction,
		Timestamp: v1.CreateOctantIntallEventTimestamp(),
		Result:    result,
		Namespace: namespace,
		Ref:       integrationName,
		Subtype:   string(v1.ArgoCDOctantInstallLogEventActionDeployIntegrationSubtype),
	}); writeLogEntryErr != nil {
		zap.L().Error("INSTALL LOG ERROR: failed to write install log event", zap.Error(writeLogEntryErr), zap.String("actionType", string(v1.CreateDeployIntegrationOctantInstallEventAction)))
	}
}

// DeleteIntegration removes a named integration from the "mdai-argocd-integration" secret in the provided namespace.
func (aci *ArgoCDIntegration) DeleteIntegration(ctx context.Context, integrationName string) error {
	namespace := aci.configuration.CurrentNamespace
	secret, err := aci.secretStore.GetSecretByNameAndNamespace(argocdSecretName, aci.configuration.CurrentNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to fetch secret %s: %w", argocdSecretName, err)
	}

	if secret.Data == nil {
		return nil
	}
	if _, exists := secret.Data[integrationName]; !exists {
		return nil
	}

	delete(secret.Data, integrationName)

	return aci.secretStore.UpdateSecret(ctx, namespace, secret)
}
