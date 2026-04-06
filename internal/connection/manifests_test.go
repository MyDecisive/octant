package connection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/mydecisive/mdai-gateway/internal/integration"
	"github.com/mydecisive/mdai-gateway/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func getNestedField(m map[string]any, keys ...string) (any, bool) {
	var current any = m
	for i, key := range keys {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}

		val, exists := currentMap[key]
		if !exists {
			return nil, false
		}

		if i == len(keys)-1 {
			return val, true
		}

		current = val
	}
	return nil, false
}

// --- Format Validation Tests ---

func TestRenderManifestFormats(t *testing.T) {
	templateData := ArgoTemplateData{
		AppName:   "format-test-app",
		Namespace: "default",
		ConnectionData: OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Logs},
		},
		DatadogIntegrationData: &integration.DataDogIntegrationData{
			APIKey: "key",
			DDUrl:  "url",
		},
	}

	formats := []ManifestOutputFormat{JSONOutputFormat, YAMLOutputFormat}

	for _, format := range formats {
		t.Run(string(format), func(t *testing.T) {
			manifests, err := renderCollectorDeploymentManifests(&templateData, format)
			require.NoError(t, err)

			expectedFiles := []string{
				fmt.Sprintf("collector.%s", format),
				fmt.Sprintf("validator.%s", format),
				fmt.Sprintf("secret.%s", format),
			}

			for _, file := range expectedFiles {
				bytes, exists := (manifests)[file]
				require.True(t, exists, "Expected file %s to exist in map", file)

				// STRICT FORMAT ENFORCEMENT
				if format == JSONOutputFormat {
					require.True(t, json.Valid(bytes), "File %s must be strictly valid JSON", file)
				}

				var parsed map[string]any
				err = yaml.Unmarshal(bytes, &parsed)
				require.NoError(t, err, "File %s should be valid %s", file, format)
				require.NotEmpty(t, parsed, "File %s should not be empty", file)
			}
		})
	}
}

// --- Individual Template Tests ---

func TestRenderArgoAppManifest(t *testing.T) {
	t.Run("Valid Argo App Configuration", func(t *testing.T) {
		templateData := ArgoTemplateData{
			AppName:   "test-app",
			Namespace: "team-a-namespace",
		}

		result, err := renderArgoAppManifest(&templateData, YAMLOutputFormat)
		require.NoError(t, err)

		var parsed map[string]any
		require.NoError(t, yaml.Unmarshal(result, &parsed))

		metadata, ok := parsed["metadata"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test-app", metadata["name"])

		spec, ok := parsed["spec"].(map[string]any)
		require.True(t, ok)
		destination, ok := spec["destination"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "team-a-namespace", destination["namespace"])
	})
}

func TestRenderSecretManifest(t *testing.T) {
	t.Run("With Sideload and Datadog Integration", func(t *testing.T) {
		templateData := ArgoTemplateData{
			AppName:        "test-app",
			IsArgoSideload: true,
			DatadogIntegrationData: &integration.DataDogIntegrationData{
				APIKey: "fake-dd-api-key",
				DDUrl:  "https://datadoghq.com",
			},
		}

		manifests, err := renderCollectorDeploymentManifests(&templateData, YAMLOutputFormat)
		require.NoError(t, err)
		secretBytes := (manifests)["secret.yaml"]

		var secret map[string]any
		require.NoError(t, yaml.Unmarshal(secretBytes, &secret))

		// Check Annotations
		secretMeta := secret["metadata"].(map[string]any)
		annotations, hasAnnotations := secretMeta["annotations"].(map[string]any)
		require.True(t, hasAnnotations)
		assert.Contains(t, annotations["argocd.argoproj.io/tracking-id"], "test-app")

		// Check StringData
		stringData, hasStringData := secret["stringData"].(map[string]any)
		require.True(t, hasStringData)
		assert.Equal(t, "fake-dd-api-key", stringData["api-key"])
		assert.Equal(t, "https://datadoghq.com", stringData["site-url"])
	})

	t.Run("Without Sideload or Datadog", func(t *testing.T) {
		templateData := ArgoTemplateData{
			AppName:                "test-app",
			IsArgoSideload:         false,
			DatadogIntegrationData: nil,
		}

		manifests, err := renderCollectorDeploymentManifests(&templateData, YAMLOutputFormat)
		require.NoError(t, err)
		secretBytes := (manifests)["secret.yaml"]

		var secret map[string]any
		require.NoError(t, yaml.Unmarshal(secretBytes, &secret))

		secretMeta := secret["metadata"].(map[string]any)
		_, hasAnnotations := secretMeta["annotations"]
		assert.False(t, hasAnnotations, "Annotations should not exist without Sideload")

		// Safely check if stringData exists but doesn't contain Datadog keys
		if stringDataRaw, hasStringData := secret["stringData"]; hasStringData && stringDataRaw != nil {
			stringData := stringDataRaw.(map[string]any)
			assert.NotContains(t, stringData, "api-key")
			assert.NotContains(t, stringData, "site-url")
		}
	})
}

func TestRenderCollectorManifest(t *testing.T) {
	t.Run("Full Configuration with Pipelines", func(t *testing.T) {
		templateData := ArgoTemplateData{
			AppName:        "test-app",
			IsArgoSideload: true,
			ConnectionData: OctantConnectionData{
				TelemetryTypes: []telemetry.MLT{telemetry.Logs, telemetry.Traces},
			},
			DatadogIntegrationData: &integration.DataDogIntegrationData{
				APIKey: "fake-key",
				DDUrl:  "fake-url",
			},
		}

		manifests, err := renderCollectorDeploymentManifests(&templateData, YAMLOutputFormat)
		require.NoError(t, err)
		collectorBytes := (manifests)["collector.yaml"]

		var otel map[string]any
		require.NoError(t, yaml.Unmarshal(collectorBytes, &otel))

		spec := otel["spec"].(map[string]any)
		_, hasEnv := spec["env"]
		assert.True(t, hasEnv, "Env block should exist for Datadog integration")

		configStr := spec["config"].(string)
		var otelConfig map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(configStr), &otelConfig))

		// Check Datadog Exporter
		apiBlock, found := getNestedField(otelConfig, "exporters", "datadog", "api")
		require.True(t, found, "Datadog API exporter should be configured")
		apiMap := apiBlock.(map[string]any)
		assert.Equal(t, "${env:DD_API_KEY}", apiMap["key"])
		assert.Equal(t, "${env:DD_SITE}", apiMap["site"])

		// Check Dynamic Pipelines
		for _, tel := range []string{"logs", "traces"} {
			receivers, found := getNestedField(otelConfig, "service", "pipelines", tel, "receivers")
			require.True(t, found, "Pipeline %s should exist", tel)
			assert.Contains(t, receivers.([]any), "datadog", "Pipeline should include datadog receiver")
		}
	})

	t.Run("Minimal Configuration without Pipelines", func(t *testing.T) {
		templateData := ArgoTemplateData{
			AppName:        "minimal-app",
			IsArgoSideload: false,
			ConnectionData: OctantConnectionData{
				TelemetryTypes: []telemetry.MLT{},
			},
			DatadogIntegrationData: nil,
		}

		manifests, err := renderCollectorDeploymentManifests(&templateData, YAMLOutputFormat)
		require.NoError(t, err)
		collectorBytes := (manifests)["collector.yaml"]

		var otel map[string]any
		require.NoError(t, yaml.Unmarshal(collectorBytes, &otel))

		spec := otel["spec"].(map[string]any)

		// Safely check if env exists but doesn't contain Datadog keys
		if envRaw, hasEnv := spec["env"]; hasEnv && envRaw != nil {
			envSlice := envRaw.([]any)
			for _, eRaw := range envSlice {
				e := eRaw.(map[string]any)
				assert.NotEqual(t, "DD_API_KEY", e["name"])
				assert.NotEqual(t, "DD_SITE", e["name"])
			}
		}

		configStr := spec["config"].(string)
		var otelConfig map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(configStr), &otelConfig))

		_, foundExporters := getNestedField(otelConfig, "exporters", "datadog", "api")
		assert.False(t, foundExporters, "Datadog API exporter should NOT be configured")

		_, foundLogs := getNestedField(otelConfig, "service", "pipelines", "logs")
		assert.False(t, foundLogs, "Logs pipeline should not exist")
	})
}

func TestRenderValidatorManifest(t *testing.T) {
	t.Run("With Signals", func(t *testing.T) {
		templateData := ArgoTemplateData{
			AppName: "test-app",
			ConnectionData: OctantConnectionData{
				TelemetryTypes: []telemetry.MLT{telemetry.Logs, telemetry.Metrics},
			},
			DatadogIntegrationData: &integration.DataDogIntegrationData{
				DDUrl: "https://datadoghq.com",
			},
		}

		manifests, err := renderCollectorDeploymentManifests(&templateData, YAMLOutputFormat)
		require.NoError(t, err)
		validatorBytes := (manifests)["validator.yaml"]

		var validator map[string]any
		require.NoError(t, yaml.Unmarshal(validatorBytes, &validator))

		spec := validator["spec"].(map[string]any)
		collectorRef := spec["collectorRef"].(map[string]any)
		assert.Equal(t, "test-app", collectorRef["name"])
	})
}

func TestCreateExportableArgoManifests(t *testing.T) {
	t.Parallel()

	connection := OctantConnectionData{
		Destinations: []OctantConnectionDestination{
			{DestinationType: "datadog", IntegrationName: "test-dd"},
		},
		Deployment: &Deployment{
			Type: ArgoManifestsDeploymentType,
		},
	}

	manifests, err := CreateExportableArgoManifests("test-namespace", "test-app", connection, YAMLOutputFormat)
	require.NoError(t, err)

	secretBytes, exists := (manifests)["secret.yaml"]
	require.True(t, exists, "Exportable secret manifest missing")

	var secret map[string]any
	require.NoError(t, yaml.Unmarshal(secretBytes, &secret))

	stringData, ok := secret["stringData"].(map[string]any)
	require.True(t, ok)

	// Ensure sensitive secrets are overwritten with placeholders
	assert.Equal(t, "<YOUR_API_KEY>", stringData["api-key"])
	assert.Equal(t, "<YOUR_DD_URL>", stringData["site-url"])
}

func TestCreateTemplateData(t *testing.T) {
	t.Parallel()

	t.Run("Multiple Destinations Error", func(t *testing.T) {
		t.Parallel()
		f := setupFixture(t)
		oc := f.build()

		connection := OctantConnectionData{
			Destinations: []OctantConnectionDestination{
				{DestinationType: "datadog"},
				{DestinationType: "dogodat"},
			},
		}

		data, err := oc.createTemplateData(context.Background(), "default", "test-app", connection)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "multiple destinations is currently unsupported")
		assert.Nil(t, data)
	})

	t.Run("Unknown Destination Type Error", func(t *testing.T) {
		t.Parallel()
		f := setupFixture(t)
		oc := f.build()

		connection := OctantConnectionData{
			Destinations: []OctantConnectionDestination{
				{DestinationType: "new-relic", IntegrationName: "nr-test"},
			},
		}

		data, err := oc.createTemplateData(context.Background(), "default", "test-app", connection)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown destination type: new-relic")
		assert.Nil(t, data)
	})

	t.Run("Datadog Integration Fetch Error", func(t *testing.T) {
		t.Parallel()
		f := setupFixture(t)
		oc := f.build()

		connection := OctantConnectionData{
			Destinations: []OctantConnectionDestination{
				{DestinationType: "datadog", IntegrationName: "broken-integration"},
			},
		}

		f.datadogMock.EXPECT().
			GetIntegrationByName(mock.Anything, "default", "broken-integration").
			Return(nil, errors.New("injected api failure")).
			Once()

		data, err := oc.createTemplateData(context.Background(), "default", "test-app", connection)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "injected api failure")
		assert.Nil(t, data)
	})
}
