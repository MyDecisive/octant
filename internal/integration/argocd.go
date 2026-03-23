package integration

import (
	"context"
	"encoding/json"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const argocdSecretName = "mdai-argocd-integration" // nolint: gosec

type ArgoCDIntegrationData struct {
	AccountToken string `json:"accountToken"`
}

func (aid *ArgoCDIntegrationData) ToFields() map[string]any {
	return map[string]any{
		"apiKey": aid.AccountToken,
	}
}

type ArgoCDIntegration struct {
	K8sClient kubernetes.Interface
}

var _ Integration[ArgoCDIntegrationData] = (*ArgoCDIntegration)(nil)

// GetIntegrations retrieves any existing integrations in the provided namespace for the "mdai-argocd-integration" secret.
func (aci *ArgoCDIntegration) GetIntegrations(ctx context.Context, namespace string) (map[string]ArgoCDIntegrationData, error) {
	secret, err := aci.K8sClient.CoreV1().Secrets(namespace).Get(ctx, argocdSecretName, metav1.GetOptions{})
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

// GetIntegrationByName retrieves the existing integration in the provided namespace for the "mdai-argocd-integration" secret, if it exists.
func (aci *ArgoCDIntegration) GetIntegrationByName(ctx context.Context, namespace, name string) (*ArgoCDIntegrationData, error) {
	secret, err := aci.K8sClient.CoreV1().Secrets(namespace).Get(ctx, argocdSecretName, metav1.GetOptions{})
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
func (aci *ArgoCDIntegration) SetIntegration(ctx context.Context, namespace, integrationName string, integrationData ArgoCDIntegrationData) error {
	jsonData, err := json.Marshal(integrationData)
	if err != nil {
		return fmt.Errorf("failed to marshal integration data: %w", err)
	}

	secret, err := aci.K8sClient.CoreV1().Secrets(namespace).Get(ctx, argocdSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Create the secret if it does not exist
			return createIntegrationSecret(ctx, aci.K8sClient, namespace, argocdSecretName, integrationName, jsonData)
		}
		return fmt.Errorf("failed to fetch secret %s: %w", argocdSecretName, err)
	}
	// Update the secret if it already exists
	return updateSecretWithIntegration(ctx, aci.K8sClient, namespace, secret, integrationName, jsonData)
}

// DeleteIntegration removes a named integration from the "mdai-argocd-integration" secret in the provided namespace.
func (aci *ArgoCDIntegration) DeleteIntegration(ctx context.Context, namespace, integrationName string) error {
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
