package integration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mydecisive/mdai-data-core/kube"
	"github.com/mydecisive/octant/internal/config"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

const githubAppSecretName = "mdai-github-app-integration" // nolint: gosec

// GitHubAppIntegrationData holds the GitHub App installation credentials and repository
// settings Octant uses to open branches/commits/pull requests against a CI/CD repository.
type GitHubAppIntegrationData struct {
	AppID          int64  `json:"appId"`
	InstallationID int64  `json:"installationId"`
	PrivateKey     string `json:"privateKey"`
	Owner          string `json:"owner"`
	Repository     string `json:"repository"`
	BaseBranch     string `json:"baseBranch"`
	BranchPrefix   string `json:"branchPrefix"`
	BasePath       string `json:"basePath"`
	CommitterName  string `json:"committerName"`
	CommitterEmail string `json:"committerEmail"`
}

type GitHubAppIntegration struct {
	secretStore   kube.SecretStore
	configuration *config.Configuration
}

var _ Integration[GitHubAppIntegrationData] = (*GitHubAppIntegration)(nil)

// NewGitHubAppIntegration returns a new instance of GitHubAppIntegration.
func NewGitHubAppIntegration(
	secretStore kube.SecretStore,
	configuration *config.Configuration,
) *GitHubAppIntegration {
	return &GitHubAppIntegration{
		secretStore:   secretStore,
		configuration: configuration,
	}
}

// GetIntegrations retrieves any existing integrations
// in the provided namespace for the "mdai-github-app-integration" secret.
func (gai *GitHubAppIntegration) GetIntegrations(_ context.Context) (map[string]GitHubAppIntegrationData, error) {
	secret, err := gai.secretStore.GetSecretByNameAndNamespace(githubAppSecretName, gai.configuration.CurrentNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil // nolint: nilnil
		}
		return nil, fmt.Errorf("failed to get secret %s: %w", githubAppSecretName, err)
	}

	integrations := make(map[string]GitHubAppIntegrationData)
	for name, data := range secret.Data {
		var payload GitHubAppIntegrationData
		if unmarshalErr := json.Unmarshal(data, &payload); unmarshalErr != nil {
			continue // Skip invalid JSON entries
		}
		integrations[name] = payload
	}

	return integrations, nil
}

// GetIntegrationByName retrieves the existing
// integration in the provided namespace for the "mdai-github-app-integration" secret, if it exists.
func (gai *GitHubAppIntegration) GetIntegrationByName(
	ctx context.Context, name string,
) (*GitHubAppIntegrationData, error) {
	secret, err := gai.secretStore.GetSecretByNameAndNamespace(githubAppSecretName, gai.configuration.CurrentNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil // nolint: nilnil
		}
		return nil, fmt.Errorf("failed to get secret %s: %w", githubAppSecretName, err)
	}

	if _, ok := secret.Data[name]; !ok {
		return nil, fmt.Errorf("integration '%s' not found", name)
	}

	var payload GitHubAppIntegrationData
	if unmarshalErr := json.Unmarshal(secret.Data[name], &payload); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal integration data: %w", unmarshalErr)
	}
	return &payload, nil
}

// SetIntegration adds or updates the "mdai-github-app-integration" secret for the provided namespace.
func (gai *GitHubAppIntegration) SetIntegration(
	ctx context.Context,
	integrationName string,
	integrationData GitHubAppIntegrationData,
) error {
	jsonData, err := json.Marshal(integrationData) // nolint: gosec // this is the whole point of the integration secret
	if err != nil {
		return fmt.Errorf("failed to marshal integration data: %w", err)
	}
	namespace := gai.configuration.CurrentNamespace
	secret, err := gai.secretStore.GetSecretByNameAndNamespace(githubAppSecretName, gai.configuration.CurrentNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Create the secret if it does not exist
			return createIntegrationSecret(
				ctx,
				gai.secretStore,
				namespace,
				integrationName,
				githubAppSecretName,
				kube.OctantIntegrationGitHubApp,
				jsonData,
			)
		}
		return fmt.Errorf("failed to fetch secret %s: %w", githubAppSecretName, err)
	}
	// Update the secret if it already exists
	return updateSecretWithIntegration(ctx, gai.secretStore, namespace, integrationName, secret, jsonData)
}

// DeleteIntegration removes a named integration from the
// "mdai-github-app-integration" secret in the provided namespace.
func (gai *GitHubAppIntegration) DeleteIntegration(ctx context.Context, integrationName string) error {
	namespace := gai.configuration.CurrentNamespace
	secret, err := gai.secretStore.GetSecretByNameAndNamespace(githubAppSecretName, gai.configuration.CurrentNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to fetch secret %s: %w", githubAppSecretName, err)
	}

	if secret.Data == nil {
		return nil
	}
	if _, exists := secret.Data[integrationName]; !exists {
		return nil
	}

	delete(secret.Data, integrationName)

	return gai.secretStore.UpdateSecret(ctx, namespace, secret)
}
