package integration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mydecisive/octant/internal/config"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const argocdSecretName = "mdai-argocd-integration" // nolint: gosec

type ArgoCDIntegrationData struct {
	APIUrl       string `json:"apiUrl"`
	AccountToken string `json:"accountToken"`
}

type ArgoCDIntegration struct {
	K8sClient     kubernetes.Interface
	configuration *config.Configuration
}

var _ Integration[ArgoCDIntegrationData] = (*ArgoCDIntegration)(nil)

// NewArgoCDIntegration returns a new instance of ArgoCDIntegration.
func NewArgoCDIntegration(k8sClient kubernetes.Interface, configuration *config.Configuration) *ArgoCDIntegration {
	return &ArgoCDIntegration{
		K8sClient:     k8sClient,
		configuration: configuration,
	}
}

// GetIntegrations retrieves any existing integrations
// in the provided namespace for the "mdai-argocd-integration" secret.
func (aci *ArgoCDIntegration) GetIntegrations(
	ctx context.Context,
) (map[string]ArgoCDIntegrationData, error) {
	secret, err := aci.K8sClient.CoreV1().Secrets(aci.configuration.CurrentNamespace).Get(ctx, argocdSecretName, metav1.GetOptions{})
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
	secret, err := aci.K8sClient.CoreV1().Secrets(aci.configuration.CurrentNamespace).Get(ctx, argocdSecretName, metav1.GetOptions{})
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
	secret, err := aci.K8sClient.CoreV1().Secrets(namespace).Get(ctx, argocdSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Create the secret if it does not exist
			return createIntegrationSecret(ctx, aci.K8sClient, namespace, integrationName, argocdSecretName, jsonData)
		}
		return fmt.Errorf("failed to fetch secret %s: %w", argocdSecretName, err)
	}
	// Update the secret if it already exists
	return updateSecretWithIntegration(ctx, aci.K8sClient, namespace, integrationName, secret, jsonData)
}

// DeleteIntegration removes a named integration from the "mdai-argocd-integration" secret in the provided namespace.
func (aci *ArgoCDIntegration) DeleteIntegration(ctx context.Context, integrationName string) error {
	namespace := aci.configuration.CurrentNamespace
	secret, err := aci.K8sClient.CoreV1().Secrets(namespace).Get(ctx, argocdSecretName, metav1.GetOptions{})
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

	if _, err = aci.K8sClient.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update secret %s after deletion: %w", argocdSecretName, err)
	}

	return nil
}
