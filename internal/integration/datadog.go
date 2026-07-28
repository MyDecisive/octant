package integration

import (
	"context"
	"encoding/json"
	"fmt"
	octantv1 "github.com/mydecisive/octant/api/v1"
	"github.com/mydecisive/octant/internal/installlog"
	"go.uber.org/zap"
	"strings"

	"github.com/mydecisive/mdai-data-core/kube"
	"github.com/mydecisive/octant/internal/config"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

const datadogSecretName = "mdai-datadog-integration" // nolint: gosec

type DataDogIntegrationData struct {
	APIKey   string `json:"apiKey"`
	SiteHost string `json:"siteHost"`
}

func (d DataDogIntegrationData) IsKnownDatadogTLD() bool {
	knownDatadogSites := []string{"datadoghq.com", "datadoghq.eu", "ddog-gov.com"}
	for _, site := range knownDatadogSites {
		if strings.Contains(d.SiteHost, site) {
			return true
		}
	}
	return false
}

type DataDogIntegration struct {
	secretStore     kube.SecretStore
	installLogStore installlog.InstallLogStore
	configuration   *config.Configuration
}

// NewDataDogIntegration returns a new instance of DataDogIntegration.
func NewDataDogIntegration(
	secretStore kube.SecretStore,
	installLogStore installlog.InstallLogStore,
	configuration *config.Configuration,
) *DataDogIntegration {
	return &DataDogIntegration{
		secretStore:     secretStore,
		installLogStore: installLogStore,
		configuration:   configuration,
	}
}

var _ Integration[DataDogIntegrationData] = (*DataDogIntegration)(nil)

// GetIntegrations retrieves any existing integrations in the provided namespace for the "octant-integration" secret.
func (ddi *DataDogIntegration) GetIntegrations(
	ctx context.Context,
) (map[string]DataDogIntegrationData, error) {
	secret, err := ddi.secretStore.GetSecretByNameAndNamespace(datadogSecretName, ddi.configuration.CurrentNamespace)
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

// GetIntegrationByName retrieves the existing integration
// in the provided namespace for the "octant-integration" secret, if it exists.
func (ddi *DataDogIntegration) GetIntegrationByName(
	_ context.Context,
	name string,
) (*DataDogIntegrationData, error) {
	secret, err := ddi.secretStore.GetSecretByNameAndNamespace(datadogSecretName, ddi.configuration.CurrentNamespace)
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
func (ddi *DataDogIntegration) SetIntegration(
	ctx context.Context,
	integrationName string,
	integrationData DataDogIntegrationData,
) error {
	jsonData, err := json.Marshal(integrationData) // nolint: gosec // what secrets...
	if err != nil {
		return fmt.Errorf("failed to marshal integration data: %w", err)
	}
	namespace := ddi.configuration.CurrentNamespace
	secret, err := ddi.secretStore.GetSecretByNameAndNamespace(datadogSecretName, ddi.configuration.CurrentNamespace)
	isNotFound := k8serrors.IsNotFound(err)
	if err != nil && !isNotFound {
		return fmt.Errorf("failed to fetch secret %s: %w", datadogSecretName, err)
	}

	if isNotFound {
		// Create the secret if it does not exist
		return createIntegrationSecret(
			ctx,
			ddi.secretStore,
			namespace,
			integrationName,
			datadogSecretName,
			kube.OctantIntegrationDatadogType,
			jsonData,
		)
	}
	// Update the secret if it already exists
	writeErr := updateSecretWithIntegration(ctx, ddi.secretStore, namespace, integrationName, secret, jsonData)

	ddi.writeInstallLogEntry(ctx, integrationName, namespace, writeErr)

	return writeErr
}

// DeleteIntegration removes a named integration from the "octant-integration" secret in the provided namespace.
func (ddi *DataDogIntegration) DeleteIntegration(ctx context.Context, integrationName string) error {
	namespace := ddi.configuration.CurrentNamespace
	secret, err := ddi.secretStore.GetSecretByNameAndNamespace(datadogSecretName, ddi.configuration.CurrentNamespace)
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

	return ddi.secretStore.UpdateSecret(ctx, namespace, secret)
}

func (ddi *DataDogIntegration) writeInstallLogEntry(
	ctx context.Context,
	integrationName string,
	namespace string,
	createErr error,
) {
	result := octantv1.FailureOctantInstallEventResult
	message := ""
	if createErr == nil {
		result = octantv1.SuccessOctantInstallEventResult
	} else {
		message = createErr.Error()
	}
	if writeLogEntryErr := ddi.installLogStore.AddInstallLogEvent(ctx, &octantv1.OctantInstallEvent{
		Action:    octantv1.CreateDestinationIntegration,
		Timestamp: octantv1.CreateOctantInstallEventTimestamp(),
		Result:    result,
		Namespace: namespace,
		Ref:       integrationName,
		Subtype:   string(octantv1.ArgoCDSubtype),
		Message:   message,
	}); writeLogEntryErr != nil {
		zap.L().Error(
			"INSTALL LOG ERROR: failed to write install log event",
			zap.Error(writeLogEntryErr),
			zap.String("actionType", string(octantv1.CreateDeployIntegration)),
		)
	}
}
