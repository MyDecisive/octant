package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const datadogSecretName = "mdai-datadog-integration" // nolint: gosec

type DataDogIntegrationData struct {
	APIKey string `json:"apiKey"`
	DDUrl  string `json:"url"`
}

func (d DataDogIntegrationData) IsKnownDatadogTLD() bool {
	knownDatadogSites := []string{"datadoghq.com", "datadoghq.eu", "ddog-gov.com"}
	for _, site := range knownDatadogSites {
		if strings.Contains(d.DDUrl, site) {
			return true
		}
	}
	return false
}

type DataDogIntegration struct {
	K8sClient kubernetes.Interface
}

var _ Integration[DataDogIntegrationData] = (*DataDogIntegration)(nil)

// GetIntegrations retrieves any existing integrations in the provided namespace for the "octant-integration" secret.
func (ddi *DataDogIntegration) GetIntegrations(ctx context.Context, namespace string) (map[string]DataDogIntegrationData, error) {
	secret, err := ddi.K8sClient.CoreV1().Secrets(namespace).Get(ctx, datadogSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil // nolint: nilnil
		}
		return nil, fmt.Errorf("failed to get secret %s: %w", datadogSecretName, err)
	}

	integrations := make(map[string]DataDogIntegrationData)
	for name, data := range secret.Data {
		var payload DataDogIntegrationData
		if unmarshalErr := json.Unmarshal(data, &payload); unmarshalErr != nil {
			continue // Skip invalid JSON entries
		}
		integrations[name] = payload
	}

	return integrations, nil
}

// GetIntegrationByName retrieves the existing integration in the provided namespace for the "octant-integration" secret, if it exists.
func (ddi *DataDogIntegration) GetIntegrationByName(ctx context.Context, namespace, name string) (*DataDogIntegrationData, error) {
	secret, err := ddi.K8sClient.CoreV1().Secrets(namespace).Get(ctx, datadogSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil // nolint: nilnil
		}
		return nil, fmt.Errorf("failed to get secret %s: %w", datadogSecretName, err)
	}

	if _, ok := secret.Data[name]; !ok {
		return nil, fmt.Errorf("integration '%s' not found", name)
	}

	var payload DataDogIntegrationData
	if unmarshalErr := json.Unmarshal(secret.Data[name], &payload); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal integration data: %w", unmarshalErr)
	}
	return &payload, nil
}

// SetIntegration adds or updates the "octant-integration" secret for the provided namespace.
func (ddi *DataDogIntegration) SetIntegration(ctx context.Context, namespace, integrationName string, integrationData DataDogIntegrationData) error {
	jsonData, err := json.Marshal(integrationData)
	if err != nil {
		return fmt.Errorf("failed to marshal integration data: %w", err)
	}

	secret, err := ddi.K8sClient.CoreV1().Secrets(namespace).Get(ctx, datadogSecretName, metav1.GetOptions{})
	isNotFound := k8serrors.IsNotFound(err)
	if err != nil && !isNotFound {
		return fmt.Errorf("failed to fetch secret %s: %w", datadogSecretName, err)
	}

	if isNotFound {
		// Create the secret if it does not exist
		return createIntegrationSecret(ctx, ddi.K8sClient, namespace, integrationName, jsonData)
	}
	// Update the secret if it already exists
	return updateSecretWithIntegration(ctx, ddi.K8sClient, namespace, integrationName, secret, jsonData)
}

// DeleteIntegration removes a named integration from the "octant-integration" secret in the provided namespace.
func (ddi *DataDogIntegration) DeleteIntegration(ctx context.Context, namespace, integrationName string) error {
	secret, err := ddi.K8sClient.CoreV1().Secrets(namespace).Get(ctx, datadogSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to fetch secret %s: %w", datadogSecretName, err)
	}

	if secret.Data == nil {
		return nil
	}
	if _, exists := secret.Data[integrationName]; !exists {
		return nil
	}

	delete(secret.Data, integrationName)

	_, err = ddi.K8sClient.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret %s after deletion: %w", datadogSecretName, err)
	}

	return nil
}
