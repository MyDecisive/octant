package connection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	argoapp "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/telemetry"
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
	t.Parallel()
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
			t.Parallel()
			manifests, err := renderCollectorDeploymentManifests(&templateData, format)
			require.NoError(t, err)

			expectedFiles := []string{
				fmt.Sprintf("lb-collector.%s", format),
				fmt.Sprintf("log-collector.%s", format),
				fmt.Sprintf("trace-collector.%s", format),
				fmt.Sprintf("observer.%s", format),
				fmt.Sprintf("hub.%s", format),
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
	t.Parallel()
	t.Run("Valid Argo App Configuration", func(t *testing.T) {
		t.Parallel()
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

func TestRenderMdaiAppManifest(t *testing.T) {
	t.Parallel()
	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		result, err := RenderMdaiAppManifest("0.9.0", "mdai")
		require.NoError(t, err)

		var parsedApp argoapp.Application
		require.NoError(t, yaml.Unmarshal(result, &parsedApp))

		sources := parsedApp.Spec.Sources
		require.Len(t, sources, 2)

		// validate mdai install version
		assert.Equal(t, "0.9.0", sources[0].TargetRevision)

		assert.Equal(t, "mdai", parsedApp.Spec.Destination.Namespace)
	})
}

func TestRenderSecretManifest(t *testing.T) {
	t.Parallel()
	t.Run("With Sideload and Datadog Integration", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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

func TestRenderLBCollectorManifest(t *testing.T) {
	t.Parallel()
	t.Run("Full Configuration with Pipelines", func(t *testing.T) {
		t.Parallel()
		templateData := ArgoTemplateData{
			AppName:        "test-app",
			Namespace:      "test-ns",
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
		collectorBytes := (manifests)["lb-collector.yaml"]

		var otel map[string]any
		require.NoError(t, yaml.Unmarshal(collectorBytes, &otel))

		labels, hasLabels := getNestedField(otel, "metadata", "labels")
		assert.True(t, hasLabels)
		assert.Equal(t, "connection-collector", labels.(map[string]any)["hub.mydecisive.ai/role"])

		spec := otel["spec"].(map[string]any)

		otelConfigRaw, hasOtelConfig := getNestedField(spec, "config")
		assert.True(t, hasOtelConfig, "OTEL config should exist")
		otelConfig := otelConfigRaw.(map[string]any)

		connectionName, hasConnectionName := getNestedField(otelConfig, "service", "telemetry", "resource", "mdai_connection")
		assert.True(t, hasConnectionName, "Connection name should be configured")
		assert.Equal(t, "test-app", connectionName)
		serviceName, hasServiceName := getNestedField(otelConfig, "service", "telemetry", "resource", "service.name")
		assert.True(t, hasServiceName, "Service name should be configured")
		assert.Equal(t, "test-app-lb-collector", serviceName)

		metricsReaders, hasMetricsReaders := getNestedField(otelConfig, "service", "telemetry", "metrics", "readers")
		assert.True(t, hasMetricsReaders, "Metrics reader should be configured")
		metricsReadersSlice := metricsReaders.([]any)
		assert.Len(t, metricsReadersSlice, 1)
		includedLabels, hasIncludedLabels := getNestedField(
			metricsReadersSlice[0].(map[string]any),
			"pull",
			"exporter",
			"prometheus",
			"with_resource_constant_labels",
			"included",
		)
		assert.True(t, hasIncludedLabels, "Prometheus pull exporter included labels should be configured")
		assert.Contains(t, includedLabels, "mdai_connection")
		assert.Contains(t, includedLabels, "service.name")

		// Check Dynamic Pipelines
		for _, tel := range []string{"logs", "traces"} {
			receivers, found := getNestedField(otelConfig, "service", "pipelines", tel, "receivers")
			require.True(t, found, "Pipeline %s should exist", tel)
			assert.Contains(t, receivers.([]any), "datadog", "Pipeline should include datadog receiver")
		}

		// Check tracealyzer exporter wiring
		tracealyzerEndpoint, hasTracealyzerEndpoint := getNestedField(otelConfig, "exporters", "otlp_grpc/tracealyzer", "endpoint")
		assert.True(t, hasTracealyzerEndpoint, "tracealyzer exporter should be configured")
		assert.Equal(t, "mdai-tracealyzer.test-ns.svc.cluster.local:4317", tracealyzerEndpoint)

		traceExporters, hasTraceExporters := getNestedField(otelConfig, "service", "pipelines", "traces", "exporters")
		require.True(t, hasTraceExporters, "Traces pipeline exporters should exist")
		assert.Contains(t, traceExporters.([]any), "otlp_grpc/tracealyzer", "Traces pipeline should include tracealyzer exporter")
	})

	t.Run("Minimal Configuration without Pipelines", func(t *testing.T) {
		t.Parallel()
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
		collectorBytes := (manifests)["lb-collector.yaml"]

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

		otelConfigRaw, hasOtelConfig := getNestedField(spec, "config")
		assert.True(t, hasOtelConfig, "OTEL config should exist")
		otelConfig := otelConfigRaw.(map[string]any)

		_, foundExporters := getNestedField(otelConfig, "exporters", "datadog", "api")
		assert.False(t, foundExporters, "Datadog API exporter should NOT be configured")

		_, foundLogs := getNestedField(otelConfig, "service", "pipelines", "logs")
		assert.False(t, foundLogs, "Logs pipeline should not exist")
	})
}

func TestRenderLogCollectorManifest(t *testing.T) {
	t.Parallel()
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
	collectorBytes := (manifests)["log-collector.yaml"]

	var otel map[string]any
	require.NoError(t, yaml.Unmarshal(collectorBytes, &otel))

	_, hasLabels := getNestedField(otel, "metadata", "labels")
	assert.True(t, hasLabels)

	spec := otel["spec"].(map[string]any)
	_, hasEnv := spec["env"]
	assert.True(t, hasEnv, "Env block should exist for Datadog integration")

	otelConfigRaw, hasOtelConfig := getNestedField(spec, "config")
	assert.True(t, hasOtelConfig, "OTEL config should exist")
	otelConfig := otelConfigRaw.(map[string]any)

	// Check Datadog Exporter
	apiBlock, found := getNestedField(otelConfig, "exporters", "datadog", "api")
	require.True(t, found, "Datadog API exporter should be configured")
	apiMap := apiBlock.(map[string]any)
	assert.Equal(t, "${env:DD_API_KEY}", apiMap["key"])
	assert.Equal(t, "${env:DD_SITE}", apiMap["site"])

	connectionName, hasConnectionName := getNestedField(otelConfig, "service", "telemetry", "resource", "mdai_connection")
	assert.True(t, hasConnectionName, "Connection name should be configured")
	assert.Equal(t, "test-app", connectionName)
	serviceName, hasServiceName := getNestedField(otelConfig, "service", "telemetry", "resource", "service.name")
	assert.True(t, hasServiceName, "Service name should be configured")
	assert.Equal(t, "test-app-log-sampling-collector", serviceName)

	metricsReaders, hasMetricsReaders := getNestedField(otelConfig, "service", "telemetry", "metrics", "readers")
	assert.True(t, hasMetricsReaders, "Metrics reader should be configured")
	metricsReadersSlice := metricsReaders.([]any)
	assert.Len(t, metricsReadersSlice, 1)
	includedLabels, hasIncludedLabels := getNestedField(
		metricsReadersSlice[0].(map[string]any),
		"pull",
		"exporter",
		"prometheus",
		"with_resource_constant_labels",
		"included",
	)
	assert.True(t, hasIncludedLabels, "Prometheus pull exporter included labels should be configured")
	assert.Contains(t, includedLabels, "mdai_connection")
	assert.Contains(t, includedLabels, "service.name")
}

func TestRenderTraceCollectorManifest(t *testing.T) {
	t.Parallel()
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
	collectorBytes := (manifests)["trace-collector.yaml"]

	var otel map[string]any
	require.NoError(t, yaml.Unmarshal(collectorBytes, &otel))

	_, hasLabels := getNestedField(otel, "metadata", "labels")
	assert.True(t, hasLabels)

	spec := otel["spec"].(map[string]any)
	_, hasEnv := spec["env"]
	assert.True(t, hasEnv, "Env block should exist for Datadog integration")

	otelConfigRaw, hasOtelConfig := getNestedField(spec, "config")
	assert.True(t, hasOtelConfig, "OTEL config should exist")
	otelConfig := otelConfigRaw.(map[string]any)

	// Check Datadog Exporter
	apiBlock, found := getNestedField(otelConfig, "exporters", "datadog", "api")
	require.True(t, found, "Datadog API exporter should be configured")
	apiMap := apiBlock.(map[string]any)
	assert.Equal(t, "${env:DD_API_KEY}", apiMap["key"])
	assert.Equal(t, "${env:DD_SITE}", apiMap["site"])

	connectionName, hasConnectionName := getNestedField(otelConfig, "service", "telemetry", "resource", "mdai_connection")
	assert.True(t, hasConnectionName, "Connection name should be configured")
	assert.Equal(t, "test-app", connectionName)
	serviceName, hasServiceName := getNestedField(otelConfig, "service", "telemetry", "resource", "service.name")
	assert.True(t, hasServiceName, "Service name should be configured")
	assert.Equal(t, "test-app-trace-sampling-collector", serviceName)

	metricsReaders, hasMetricsReaders := getNestedField(otelConfig, "service", "telemetry", "metrics", "readers")
	assert.True(t, hasMetricsReaders, "Metrics reader should be configured")
	metricsReadersSlice := metricsReaders.([]any)
	assert.Len(t, metricsReadersSlice, 1)
	includedLabels, hasIncludedLabels := getNestedField(
		metricsReadersSlice[0].(map[string]any),
		"pull",
		"exporter",
		"prometheus",
		"with_resource_constant_labels",
		"included",
	)
	assert.True(t, hasIncludedLabels, "Prometheus pull exporter included labels should be configured")
	assert.Contains(t, includedLabels, "mdai_connection")
	assert.Contains(t, includedLabels, "service.name")
}

func TestRenderValidatorManifest(t *testing.T) {
	t.Parallel()
	t.Run("With Signals", func(t *testing.T) {
		t.Parallel()
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

func TestRenderObserverManifest(t *testing.T) {
	t.Parallel()
	t.Run("With Signals", func(t *testing.T) {
		t.Parallel()
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
		bytes := (manifests)["observer.yaml"]

		var data map[string]any
		require.NoError(t, yaml.Unmarshal(bytes, &data))

		spec := data["spec"].(map[string]any)
		assert.Len(t, spec["observers"], 2)
		assert.Contains(t, spec, "observerResource")
	})
}

func TestRenderHubManifest(t *testing.T) {
	t.Parallel()
	t.Run("With Signals", func(t *testing.T) {
		t.Parallel()
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
		hubBytes := (manifests)["hub.yaml"]

		var hub map[string]any
		require.NoError(t, yaml.Unmarshal(hubBytes, &hub))

		spec := hub["spec"].(map[string]any)
		if variablesRaw, hasVar := spec["variables"]; hasVar && variablesRaw != nil {
			varSlice := variablesRaw.([]any)
			logRatio := varSlice[0].(map[string]any)
			assert.Equal(t, "logs_ratio_number", logRatio["key"])
			assert.Equal(t, "string", logRatio["dataType"])
			assert.Contains(t, "LOGS_RATIO_NUMBER", logRatio["serializeAs"].([]any)[0].(map[string]any)["name"])
			logErr := varSlice[1].(map[string]any)
			assert.Equal(t, "logs_persist_errors", logErr["key"])
			assert.Equal(t, "boolean", logErr["dataType"])
			assert.Contains(t, "LOGS_PERSIST_ERRORS", logErr["serializeAs"].([]any)[0].(map[string]any)["name"])
			traceRatio := varSlice[2].(map[string]any)
			assert.Equal(t, "traces_ratio_number", traceRatio["key"])
			assert.Equal(t, "string", traceRatio["dataType"])
			assert.Contains(t, "TRACES_RATIO_NUMBER", traceRatio["serializeAs"].([]any)[0].(map[string]any)["name"])
			traceErr := varSlice[3].(map[string]any)
			assert.Equal(t, "traces_persist_errors", traceErr["key"])
			assert.Equal(t, "boolean", traceErr["dataType"])
			assert.Contains(t, "TRACES_PERSIST_ERRORS", traceErr["serializeAs"].([]any)[0].(map[string]any)["name"])
		}
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

	_, hasLBCollector := manifests["lb-collector.yaml"]
	assert.True(t, hasLBCollector, "lb-collector.yaml should exist")
	_, hasLogCollector := manifests["log-collector.yaml"]
	assert.True(t, hasLogCollector, "log-collector.yaml should exist")
	_, hasTraceCollector := manifests["trace-collector.yaml"]
	assert.True(t, hasTraceCollector, "trace-collector.yaml should exist")
	_, hasHub := manifests["hub.yaml"]
	assert.True(t, hasHub, "hub.yaml should exist")
	_, hasObserver := manifests["observer.yaml"]
	assert.True(t, hasObserver, "observer.yaml should exist")
	_, hasSecret := manifests["secret.yaml"]
	assert.True(t, hasSecret, "secret.yaml should exist")
	_, hasValidator := manifests["validator.yaml"]
	assert.True(t, hasValidator, "validator.yaml should exist")
	_, hasArgoApp := manifests["argo-app.yaml"]
	assert.True(t, hasArgoApp, "argo-app.yaml should exist")

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

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		f := setupFixture(t)
		oc := f.build()

		connection := OctantConnectionData{
			Destinations: []OctantConnectionDestination{
				{DestinationType: "datadog", IntegrationName: "broken-integration"},
			},
			Deployment: &Deployment{
				Type: ArgoManifestsDeploymentType,
			},
		}

		f.datadogMock.EXPECT().
			GetIntegrationByName(mock.Anything, "default", "broken-integration").
			Return(&integration.DataDogIntegrationData{}, nil).
			Once()

		data, err := oc.createTemplateData(context.Background(), "default", "test-app", connection)
		require.NoError(t, err)
		require.NotNil(t, data)
	})
}
